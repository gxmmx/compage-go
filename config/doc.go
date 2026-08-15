// Package config loads a typed configuration struct from defaults, one file,
// environment variables, changed flags, and explicit runtime overrides. Sources
// resolve in this order: default < file < env < flag < set.
//
// A successful load publishes an immutable snapshot. Values returns a defensive
// copy; Source, Origin, and HasSource report its provenance. Load and Set validate
// the entire prospective snapshot before publication, so a failed operation never
// changes a previously loaded configuration.
//
// Files are optional and use the target selected by WithFile or the non-empty
// environment variable selected by WithConfigEnv. JSON, TOML, and YAML are
// supported. Unknown file keys are errors. Save writes a fresh document containing
// only save-tagged values whose effective source is default, file, or set; flag and
// environment values remain transient. Sensitive defaults are never written, and
// sensitive persisted values cause Save to use mode 0600.
//
// For example:
//
//	type App struct {
//		Port int `default:"8080" env:"MYAPP_PORT" save:"true"`
//	}
//	cfg, err := config.Load[App](config.WithFile("./app.toml"))
//	if err != nil { /* handle configuration failure */ }
//	port := cfg.Values().Port
package config
