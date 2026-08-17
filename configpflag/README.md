# configpflag

`configpflag` adapts a `pflag.FlagSet` to `config.FlagSource` without making
the core `config` package depend on pflag.

```go
flags := pflag.NewFlagSet("app", pflag.ContinueOnError)
flags.Int("port", 8080, "listen port")

cfg, err := config.Load[AppConfig](
    config.WithFlagSource(configpflag.Source{Flags: flags}),
)
if err != nil { return err }
_ = cfg
```

Only flags explicitly changed by the caller win over lower-priority config
sources. `pflag.StringSlice` flags are supported; config decodes pflag's
comma-separated string-slice representation.
