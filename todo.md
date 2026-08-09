# compage-go — Design Issues

Tracked issues from initial code review. Address one at a time, each as its own commit on `dev`.

## Console

- [x] **1. Color handling** — Extracted to standalone `style/` package built around a `Styler` that captures the TTY/`NO_COLOR`/suppress decision at init. Terminal detection split to `term/`.
- [x] **2. generateRunID uses crypto/rand** — Log correlation IDs don't need cryptographic randomness. Switch to `math/rand/v2`.
- [x] **3. Prompter type assertion** — Resolved by the style redesign: `NewPrompter` now builds its own `style.New(...)` from config instead of reaching into `p.(*printer).color`.
- [x] **4. Printer write serialization** — Added a `*sync.Mutex` shared by pointer across all printers derived via `WithIndent`/`WithTextColor`. Locked in `writeLine`/`writeRaw`/`Table`. The prompter now embeds the concrete `*printer` and routes prompt output through `writeRaw`, so interactive prompts share the same lock (and its duplicate `outW`/`styler` fields were removed). Covered by a `-race` concurrency test.
- [x] **5. fanHandler error awareness** — Documented the intentional silent drop of child write errors (infallible logger; one failing sink must not abort the others).
- [x] **9. Package hierarchy** — Split `console` into sibling subpackages `console/logger` and `console/printer` (prompter folded into printer). Logger and printer are now equals, each with its own `New` and package-local `Option`/`WithLevel` (no name clash, no supertype). Root `console` keeps only `ParseLevel` and the exported hint contract (`HintPrefix`/`HintIndent`/`HintSuccess`) shared by both. Printer is bound to stdout/stderr with `WithOutToErr`/`WithErrToOut` redirects (default out→stdout, err→stderr) and an unexported writer override for tests; prompter is stdin-only with an unexported input override. Dropped `PrinterOptFor`/`PrompterOption`/public `WithInput`/`WithOutTo`/`WithErrTo`.

## Config

- [ ] **6. required validation uses zero-value** — `required:"true"` fails for fields that legitimately resolve to `0`/`""`/`false`. Check source resolution instead of zero-value.
- [ ] **7. Concurrency on Set()/Values()** — No locking. Add `sync.RWMutex` for safe concurrent access.
- [ ] **8. Remove viper dependency** — Viper pulls 15 deps for minimal surface usage. Replace with BurntSushi/toml + go.yaml.in/yaml/v3 + direct os.Getenv + pflag.Lookup. Eliminates dual-viper fragility, case-normalization, full re-unmarshal on Set().
