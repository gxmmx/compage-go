# configv2 — Design Doc

A ground-up rewrite of the `config` package that drops the viper/mapstructure/pflag
dependency chain in favor of our own parsing, layering, and decoding. We use only a
small slice of viper today, and pay for it with a large transitive dependency set and
the dual-viper source-tracking hack.

This document is the plan. **No implementation yet** — we build it in phases, each a
focused step, referencing the current `config/` package for logic and ordering.

> Prior console-package work and the v1 config review (items 1–9) are complete and
> live in git history. This file is now solely the configv2 plan.

---

## 1. Goals & principles

- **No viper, no mapstructure, no pflag in core.** Replace with `BurntSushi/toml`,
  `go.yaml.in/yaml/v3`, `encoding/json`, `os.Getenv`, and our own reflection decoder.
- **Explicit layers, explicit provenance.** Every value knows which source produced
  it. Source tracking is a property of the layer model, not a second parse of the file.
- **Loader-agnostic core.** File/env/flag loaders feed a common layer store. The core
  never imports a flag library; adapters convert external flag sets into our generic
  shape.
- **Deterministic, testable, concurrency-safe.** Pure functions where possible; a
  single mutex guards mutable state.
- **Same ergonomics as v1** for the happy path: generic `Config[T]`, struct tags,
  `Load`/`Values`/`Source`/`Set`/`Save`.

---

## 2. Architecture overview

Two data flows:

**Load (sources → struct):**
```
registry(T)        defaults ─┐
   │                env ─────┤
   ▼                flags ───┤→ layer store (ordered, each labeled) → resolve(key)
schema/fieldMeta    files ───┤        │                                   │
                    set ─────┘        ▼                                   ▼
                              precedence + provenance            decode → T + validate
```

**Save (struct/store → file):**
```
layer store + registry(save/sensitive tags) → select writable keys
   → route to write-target file → encode (toml/yaml/json) → atomic write
```

The **layer store** is the centerpiece and what replaces both vipers: an ordered set
of named layers (`default`, `file:system`, `file:user`, `env`, `flag`, `set`, …). A
lookup walks layers by precedence and returns `(value, winningLayer)`. Provenance is
just the winning layer's label — no second parse needed.

---

## 3. Components (the meaningful parts)

Grouped by responsibility. Each is a buildable unit.

### A. Config logging (`logBuffer`) — *was diagnostics*
Rename `diagnostics.go`'s `diagBuffer` to a **config log buffer**. Buffers `slog.Record`s
during `Load`/`Set`/`Save` and flushes to a caller-provided `*slog.Logger`.
- Records: tier registration, file discovery/parse results, per-key resolution +
  winning source, save decisions. **Redact `sensitive` values.**
- API: `log(msg, args...)`, `flush(*slog.Logger)`; keep the caller-PC capture so line
  info is useful. Mirrors `console/logger`'s prelogger pattern.
- *Ref:* `config/diagnostics.go`, and every `c.diag.log(...)` call site in v1.

### B. Struct registry & tags
Reflect `T` into `[]fieldMeta`. Largely reusable from v1, minus viper.
- Tags: `cfg` (key), `flag`, `env`, `default`, `required`, `sensitive`, `save`.
- Recurse nested `cfg` structs; build dotted keys, env names (prefix + suffix),
  validate duplicate flags.
- *Open:* keep tag names as-is? add a tag to target which file/layer a field saves to
  (see §5.2)? supported field types drive the decoder (§ E).
- *Ref:* `config/registry.go` (`buildRegistry`, `validateRegistry`, `fieldMeta`).

### C. Parsers / encoders (format ↔ `map[string]any`)
Thin adapters over the format libraries. Decode bytes → nested `map[string]any`;
encode `map[string]any` → bytes. Both directions (Save needs encode).
- toml → `BurntSushi/toml`; yaml → `go.yaml.in/yaml/v3`; json → `encoding/json`.
- Normalize to a common nested-map shape and flatten to dotted keys for the store.
- *Ref:* v1 delegated this to viper (`ReadInConfig`, `WriteConfigAs`).

### D. Layer store & resolution engine  ← **replaces dual-viper**
An ordered list of layers; each layer is `{label string, values map[string]any}`.
- `resolve(key) → (value, label)`: first hit walking high→low precedence.
- Precedence (low→high), matching v1: `default < file(s) < env < flag < set`.
- Provenance = winning label. Kills the `filev` second-parse entirely.
- *Open:* how multi-file layers order among themselves (see §5.1).
- *Ref:* v1's `Config.v` + `Config.filev`, `determineSource`, `buildSourceMap`.

### E. Decoder (`map`/store → struct via `cfg` tag)  ← **replaces mapstructure**
Reflection walk of `T`: for each `fieldMeta`, get resolved value, coerce to the field's
type, set it.
- **Initial supported types:** `string`, `bool`, `int`, `int64`, `uint`, `uint64`,
  `float64`, `time.Duration` (parse `"5s"`), `[]string` (comma-split for env/flag,
  native list from file). Phase-2: `float32`, other int widths, `[]int`,
  `map[string]string`, `time.Time`, pointer-for-optional.
- **Failure policy (decided): fail fast.** Absent value → use default (not a failure).
  Present-but-uncoercible value → hard error at `Load` with field/key/source/value/
  expected-type. Never silently substitute a default for a bad value.
- *Ref:* v1 `c.v.Unmarshal(..., TagName:"cfg")` at `config/config.go:177,225`.

### F. Loaders (produce layers)
- **Defaults** from registry → base layer.
- **Env** per `fieldMeta.env` via `os.LookupEnv` → env layer (also drives provenance).
- **Flags** via the generic flag source (§ G) → flag layer (only "changed" flags win).
- **Files** via discovery/modes (§5.1) → one or more file layers.
- **Set** (runtime) → top layer.
- *Ref:* v1 `Load()` tier wiring in `config/config.go:77–174`.

### G. Flag adapter (generic core + pflag generator)  ← **focus area, §5.3**
### H. File discovery & loading modes  ← **focus area, §5.1**
### I. Save / persistence with source control  ← **focus area, §5.2**

### J. Required validation
Validate on **whether a source resolved the field** (winning label != "" / not just
default), NOT zero-value. Fixes the v1 bug where `required` + legit `0`/`""`/`false`
failed.
- *Ref:* v1 `isZero(c.v.Get(...))` at `config/config.go:186–197`.

### K. Concurrency
Single `sync.RWMutex` on the `Config[T]`: read-lock `Values`/`Source`/`Path`,
write-lock `Set`/`Save`/`Load`. Fixes v1's unlocked `Set`/`Values`.

### L. Public API
`New[T]`/`Load[T]`, `Values`, `Source`, `Set`, `Save`, `Path(s)`, `FlushLog`.
Keep v1's shape so migration is mechanical.

---

## 4. What we deliberately drop from viper

- Live-watch/reload, remote providers, sub-configs, `mapstructure` hooks we don't use,
  case-normalization surprises, and the full re-unmarshal on every `Set()`.

---

## 5. Focus areas (need deeper planning — decisions flagged)

### 5.1 File discovery & loading modes  *(component H)*
v1 supports three *discovery* modes (explicit path / search paths / none). v2 must also
support multi-file **composition** modes. Candidate set (to be finalized in planning):

1. **Single file** — explicit path, `ConfigEnv` path, or first-found in search paths
   (v1 behavior).
2. **System + user (merge)** — load system config, then deep-merge user config on top
   (user keys win per-key; unset user keys fall back to system).
3. **User only** — ignore system even if present.
4. **System vs user (whole-file precedence, no merge)** — if a user file exists, it
   fully replaces system; else system. No per-key merge.
5. **Piecemeal (merge all found)** — every file found across configured locations is
   loaded and merged as its own layer, in a defined order (all-found, not first-found).

**Open decisions:**
- Merge semantics: deep per-key merge vs. whole-file replace — per mode, or configurable?
- How modes are expressed in the API (an explicit mode enum + role-tagged paths, e.g.
  `WithSystemPath`/`WithUserPath`, vs. a general ordered `WithLayer` list?).
- How each file maps to a layer label for provenance (§ D).
- "Some different types" of config the user mentioned — clarify in planning.
- *Ref:* `config/resolve.go` (`resolveFileSetup`, `storePath`, `inferFileType`).

### 5.2 Registry with source control & Save  *(components I + D)*
**Settled (see §8 #6):** the `save`-marked fields are the file surface; `Save()` writes
each at its resolved value (defaults included for exposed fields); unmarked fields are
never written; `sensitive`/transient are guarded. No scaffold mode. So *what* and
*whether-defaults* are decided.

What remains is **where**, in multi-file modes: when a value could belong to more than
one file (e.g. system + user both loaded, user overrides three keys), which file does a
changed key save back to?
- Provenance from the layer store (§ D) gives each key's origin file → the natural
  round-trip target (a key that came from the user file saves back to the user file).
- The tricky case (from the user): a key whose value is currently a system-file value or
  a default, now changed by the user — do **not** rewrite system config; instead write
  the new key into the **user** file as an override. So the write target for a change is
  "the user/highest-writable layer," not necessarily the origin layer.
- Mechanics reused from v1: atomic temp+rename; `0600` if any sensitive field written,
  else `0644`.

**Open decisions (for the P9 dive):**
- Designating the writable target file/layer (role-tagged, e.g. the "user" layer is
  writable; system is read-only?), and whether a field can override where it writes.
- Which *sane subset* of multi-file cases we support — not every combination.
- *Ref:* `config/config.go:236–318` (`Save`, `writePath`), `resolve.go:storePath`.

### 5.3 Flags: generic core + pflag generator  *(component G)*
Core must work for programs **not** using cobra/pflag, but offer a smooth path for those
that do.
- **Generic flag source interface** in core: something that reports, per registered
  name, `(value any, changed bool)` — no pflag import.
- **pflag adapter** (separate subpackage, e.g. `config/pflagx`): wraps a
  `*pflag.FlagSet` into the generic source. Only this subpackage imports `spf13/pflag`.
- **Generator (reverse direction):** derive flag *definitions* from the registry so a
  cobra command can register flags straight from the struct (name/default/usage from
  tags). Nice-to-have; sequence after the reader side works.

**Open decisions:**
- Exact generic interface shape (pull-by-name vs. push a snapshot map).
- Whether the generator emits pflag definitions directly or a neutral spec the adapter
  turns into pflags.
- *Ref:* v1 `WithFlags(*pflag.FlagSet)` (`options.go:64`), bind loop
  (`config.go:148–174`), `determineSource` flag branch (`config.go:356–360`).

---

## 8. Deep-dive: schema definition (the struct the user writes)  ← **ACTIVE**

The registry (component B) is built by reflecting the user's struct, so the tag design
is the first thing to lock. Strawman:

```go
type Config struct {
    Server struct {
        Port int    `cfg:"port" env:"PORT" flag:"port" default:"8080"`
        Host string `cfg:"host" default:"0.0.0.0"`
    } `cfg:"server"`
    APIKey   string `cfg:"api_key" env:"API_KEY" sensitive:"true"`
    LogLevel string `cfg:"log_level" default:"info"`
}
```

**Status: resolved** (except #6's coupled §5.2 detail and #8/#9 deferrals). Tag set is
locked to the v1 names — `cfg`, `env`, `flag`, `default`, `required`, `sensitive`,
`save` — with new derivation defaults below. §5.2 may add a save-target tag (#9).

Decision axes (recommendation → in **bold**; ✅ = decided, ⇦ = needs sign-off):

1. ✅ **Tag form.** Separate tags (`cfg:"" env:"" ...`) vs. one consolidated tag. →
   **keep separate** (v1, idiomatic, readable, lets other tools read the same struct).
2. ✅ **Key derivation.** v1 requires `cfg` to opt-in; untagged fields skipped. →
   **auto-derive key from field name (snake_case) when `cfg` omitted; `cfg:"-"` to
   exclude.** Less boilerplate, `encoding/json` feel.
   - **Must use an acronym-aware camel→snake converter**, not naive per-capital
     splitting (else `APIKey` → `a_p_i_key`). Boundary rules: (1) lower/digit → upper;
     (2) upper → upper-followed-by-lower. Gives `APIKey`→`api_key`, `HTTPServer`→
     `http_server`, `UserID`→`user_id`, `ID`→`id`. No hardcoded initialism list.
   - Edge cases the algorithm mangles (`IPv4`→`ip_v4`, `OAuth2`→`o_auth2`) are handled
     by the explicit `cfg:"..."` escape hatch. Env/flag derivation (#3/#4) reuse the
     same converter.
3. ✅ **Env derivation.** → **auto-derive `PREFIX_PATH` from the key; `env:"-"` opts out,
   explicit `env:"NAME"` overrides.** Env is expected on ~every field.
4. ✅ **Flag derivation.** → **opt-in only (explicit `flag:"name"`).** You don't want 50
   auto-generated CLI flags; flags are a curated subset.
5. ✅ **Nesting.** cfg-tagged nested struct → prefix namespace (v1). Anonymous embedded
   struct → **flatten into parent (no prefix).**
6. ✅ **`save` = the file surface (opt-in).** `save:"true"` means "this field belongs in
   the config file." This is a *curation* decision only the developer can make — see the
   log example: `log.level` is exposed (written at its default `"info"`), `log.timestamp`
   stays out despite also having a default. (Reverses an earlier opt-out call; the
   curated-surface requirement makes opt-in correct.)
   - **Unmarked fields** are never *written* by the tool, but remain **loadable** — a
     power user can add one to the file by hand and it overrides the default.
   - **`Save()` writes exactly the save-marked fields at their resolved value.** On a
     fresh run (all defaults) this materializes the curated file (`log.level="info"`,
     nothing else); after a `set`, it writes the new value. Same operation both times —
     **no separate scaffold mode, no "write defaults?" flag.**
   - "Freezing defaults" is a non-issue: only non-curated fields could be frozen, and
     those are never written. For an exposed field, writing its default is the intent.
   - **Secrets/transient still guarded:** `sensitive` fields aren't written unless
     `set`/from-file (perms tighten to 0600); env/flag-sourced values aren't frozen in.
7. ✅ **`required` + `default`.** Contradictory (a default always satisfies required). →
   **registry-build error** if a field is both.
8. **Optional / unset.** Support pointer fields (`*int`) → nil when no source provides a
   value, cleanly separating "unset" from "zero". → **phase-2**, note now; interacts
   with required validation (J).
9. **Save-target tag.** Whether a field needs a tag to say which file it belongs to →
   **defer to §5.2**; may add one tag member then.

## 6. Phased build order

Each phase builds on the prior and can be tested in isolation. Order mirrors v1's
`Load` sequence where sensible.

- **P0 — Scaffolding.** New package (naming TBD: `configv2` dir vs. replace `config`).
  Skeleton types, no logic. Decide package layout.
- **P1 — Config logging (A).** Standalone buffer + flush. Everything downstream logs
  through it.
- **P2 — Registry & tags (B).** Reflection → `fieldMeta`, dup-flag validation. Reuse v1.
- **P3 — Parsers/encoders (C).** toml/yaml/json ↔ `map[string]any`, both directions.
- **P4 — Layer store & resolution (D).** Ordered layers, precedence, provenance. The
  dual-viper replacement.
- **P5 — Loaders: defaults + env + generic flags (F, G-core).** Feed layers; flag source
  interface (no pflag yet).
- **P6 — File discovery & modes (H, §5.1).** Single-file first, then composition modes.
- **P7 — Decoder (E).** Store → `T` with type coercion via `cfg`.
- **P8 — Load orchestration + required validation (J) + concurrency (K).** Wire P1–P7
  into `Load()`; source-based required check; mutex.
- **P9 — Save with source control (I, §5.2).** Write-target selection, save/sensitive
  rules, atomic write.
- **P10 — pflag adapter + flag generator (G, §5.3).** Separate subpackage.
- **P11 — Public API polish (L), README, tests, migrate callers.**

---

## 7. Decisions

**Settled:**
- ✅ **Package layout:** `configv2/` sibling dir; keep `config` (v1) working during the port.
- ✅ **Decoder types + failure policy:** initial type list fixed; fail-fast on
  present-but-invalid, default only fills absent (see § E).

**Still open (by phase):**
- **Schema / tag structure (§8)** — *active deep-dive; gates the registry.*
- Loading modes: final set + per-mode merge semantics + API surface (§5.1) — *deep-dive when we reach P6.*
- Multi-file Save source-control: how to know a value's origin file and route the save
  correctly; sane subset of cases, not all combinations (§5.2) — *dedicated deep-dive with P9.*
- Generic flag-source interface shape + generator output form (§5.3).
