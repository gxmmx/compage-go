# compage-go — Design Issues

Tracked issues from initial code review. Address one at a time, each as its own commit on `dev`.

## Console

- [x] **1. Color handling** — Extracted to standalone `style/` package built around a `Styler` that captures the TTY/`NO_COLOR`/suppress decision at init. Terminal detection split to `term/`.
- [x] **2. generateRunID uses crypto/rand** — Log correlation IDs don't need cryptographic randomness. Switch to `math/rand/v2`.
- [x] **3. Prompter type assertion** — Resolved by the style redesign: `NewPrompter` now builds its own `style.New(...)` from config instead of reaching into `p.(*printer).color`.
- [x] **4. Printer write serialization** — Added a `*sync.Mutex` shared by pointer across all printers derived via `WithIndent`/`WithTextColor`. Locked in `writeLine`/`writeRaw`/`Table`. Covered by a `-race` concurrency test.
- [x] **5. fanHandler error awareness** — Documented the intentional silent drop of child write errors (infallible logger; one failing sink must not abort the others).

## Config

- [ ] **6. required validation uses zero-value** — `required:"true"` fails for fields that legitimately resolve to `0`/`""`/`false`. Check source resolution instead of zero-value.
- [ ] **7. Concurrency on Set()/Values()** — No locking. Add `sync.RWMutex` for safe concurrent access.
- [ ] **8. Remove viper dependency** — Viper pulls 15 deps for minimal surface usage. Replace with BurntSushi/toml + go.yaml.in/yaml/v3 + direct os.Getenv + pflag.Lookup. Eliminates dual-viper fragility, case-normalization, full re-unmarshal on Set().
