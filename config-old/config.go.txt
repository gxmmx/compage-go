package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// Config holds the resolved configuration and provides management methods.
type Config[T any] struct {
	opts    options
	v       *viper.Viper
	filev   *viper.Viper
	fields  []fieldMeta
	sources map[string]string
	setKeys map[string]bool
	values  *T
	loaded  bool
	diag    diagBuffer
}

// New creates a Config instance without loading. Call Load() to resolve values.
func New[T any](opts ...Option) *Config[T] {
	c := &Config[T]{
		sources: make(map[string]string),
		setKeys: make(map[string]bool),
	}
	for _, o := range opts {
		o(&c.opts)
	}
	return c
}

// Load is a convenience that creates a Config and loads it in one call.
func Load[T any](opts ...Option) (*Config[T], error) {
	c := New[T](opts...)
	if err := c.Load(); err != nil {
		return c, err
	}
	return c, nil
}

// Load resolves configuration from all tiers and populates Values().
func (c *Config[T]) Load() error {
	c.v = viper.New()

	var t T
	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}

	c.opts.sanitize()

	envPrefix := c.opts.envPrefix
	if envPrefix != "" && !strings.HasSuffix(envPrefix, "_") {
		envPrefix += "_"
	}

	fields, err := buildRegistry(rt, "", "", envPrefix, &c.diag)
	if err != nil {
		return err
	}
	c.fields = fields

	if err := validateRegistry(fields); err != nil {
		return err
	}

	for _, f := range fields {
		if f.hasDefault {
			c.v.SetDefault(f.key, f.defaultVal)
			c.diag.log("tier: default registered",
				"key", f.key,
				"value", f.defaultVal,
			)
		}
	}

	setup, err := resolveFileSetup(&c.opts, &c.diag)
	if err != nil {
		return err
	}

	switch setup.mode {
	case fileModeExplicit:
		c.v.SetConfigFile(setup.path)
		c.v.SetConfigType(setup.fileType)

	case fileModePaths:
		c.v.SetConfigName(c.opts.name)
		c.v.SetConfigType(c.opts.fileType)
		for _, p := range c.opts.paths {
			c.v.AddConfigPath(p)
		}
	}

	if setup.mode != fileModeNone {
		if err := c.v.ReadInConfig(); err != nil {
			var notFound viper.ConfigFileNotFoundError
			if !errors.As(err, &notFound) {
				return fmt.Errorf("reading config file: %w", err)
			}
			c.diag.log("tier: config file not found (graceful)")
		} else {
			c.diag.log("tier: config file loaded", "path", c.v.ConfigFileUsed())

			c.filev = viper.New()
			switch setup.mode {
			case fileModeExplicit:
				c.filev.SetConfigFile(setup.path)
				c.filev.SetConfigType(setup.fileType)
			case fileModePaths:
				c.filev.SetConfigName(c.opts.name)
				c.filev.SetConfigType(c.opts.fileType)
				for _, p := range c.opts.paths {
					c.filev.AddConfigPath(p)
				}
			}
			_ = c.filev.ReadInConfig()
		}
	}

	for _, f := range fields {
		if f.env == "" {
			continue
		}
		if err := c.v.BindEnv(f.key, f.env); err != nil {
			return &ParseError{
				Field: f.structPath,
				Key:   f.key,
				Err:   fmt.Errorf("binding env var %s: %w", f.env, err),
			}
		}
		c.diag.log("tier: env var bound",
			"key", f.key,
			"env_var", f.env,
		)
	}

	if c.opts.flags != nil {
		for _, f := range fields {
			if f.flag == "" {
				continue
			}
			pf := c.opts.flags.Lookup(f.flag)
			if pf == nil {
				c.diag.log("tier: flag not found in flagset (skipped)",
					"key", f.key,
					"flag", f.flag,
				)
				continue
			}
			if err := c.v.BindPFlag(f.key, pf); err != nil {
				return &ParseError{
					Field: f.structPath,
					Key:   f.key,
					Err:   fmt.Errorf("binding flag %s: %w", f.flag, err),
				}
			}
			c.diag.log("tier: flag bound",
				"key", f.key,
				"flag", f.flag,
				"changed", pf.Changed,
			)
		}
	}

	var result T
	if err := c.v.Unmarshal(&result, viper.DecoderConfigOption(func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "cfg"
	})); err != nil {
		return fmt.Errorf("unmarshaling config: %w", err)
	}
	c.values = &result

	c.buildSourceMap()

	var missing []string
	for _, f := range fields {
		if !f.required {
			continue
		}
		if isZero(c.v.Get(f.key)) {
			missing = append(missing, f.key)
		}
	}
	if len(missing) > 0 {
		return &ValidationError{Missing: missing}
	}

	c.loaded = true
	return nil
}

// Values returns the resolved configuration struct.
func (c *Config[T]) Values() *T {
	return c.values
}

// Source returns the provenance of a key's value: "set", "flag", "env", "file",
// "default", or "" (unset).
func (c *Config[T]) Source(key string) string {
	return c.sources[key]
}

// Set overrides a key's value at runtime with source "set" (highest precedence).
func (c *Config[T]) Set(key string, val any) error {
	if !c.hasKey(key) {
		return fmt.Errorf("%w: %s", ErrUnknownKey, key)
	}
	c.v.Set(key, val)
	c.sources[key] = "set"
	c.setKeys[key] = true
	c.diag.log("set: key updated", "key", key)

	var result T
	if err := c.v.Unmarshal(&result, viper.DecoderConfigOption(func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "cfg"
	})); err != nil {
		return fmt.Errorf("unmarshaling after set: %w", err)
	}
	c.values = &result
	return nil
}

// Save persists configuration to file. Returns ErrNoConfigFile if no write
// target is configured.
func (c *Config[T]) Save() error {
	path := c.writePath()
	if path == "" {
		return ErrNoConfigFile
	}

	sv := viper.New()
	sv.SetConfigType(inferFileType(path, c.opts.fileType))

	hasSensitive := false
	for _, f := range c.fields {
		if !f.save {
			continue
		}

		if f.sensitive {
			hasSensitive = true
		}

		source := c.sources[f.key]

		if source == "env" || source == "flag" {
			c.diag.log("save: field excluded (transient source)",
				"key", f.key,
				"source", source,
			)
			continue
		}

		if f.sensitive && source != "set" && source != "file" {
			c.diag.log("save: sensitive field excluded (not set/file)",
				"key", f.key,
				"source", source,
			)
			continue
		}

		sv.Set(f.key, c.v.Get(f.key))
		c.diag.log("save: field written",
			"key", f.key,
			"source", source,
		)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory %s: %w", dir, err)
	}

	perms := os.FileMode(0644)
	if hasSensitive {
		perms = 0600
	}

	ft := inferFileType(path, c.opts.fileType)
	tmp, err := os.CreateTemp(dir, ".config-*."+ft)
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()

	if err := sv.WriteConfigAs(tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing config file %s: %w", path, err)
	}

	if err := os.Chmod(tmpPath, perms); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting permissions on %s: %w", path, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp config to %s: %w", path, err)
	}

	c.diag.log("save: complete",
		"path", path,
		"permissions", fmt.Sprintf("%04o", perms),
	)
	return nil
}

// Path returns the resolved config file path, or "" if no file was found/configured.
func (c *Config[T]) Path() string {
	return c.v.ConfigFileUsed()
}

// FlushDiagnostics replays all buffered debug records to the target logger,
// then clears the buffer.
func (c *Config[T]) FlushDiagnostics(target *slog.Logger) {
	c.diag.flush(target)
}

func (c *Config[T]) buildSourceMap() {
	for _, f := range c.fields {
		c.sources[f.key] = c.determineSource(f)
		src := c.sources[f.key]
		if f.sensitive {
			c.diag.log("load: field resolved",
				"key", f.key,
				"source", src,
				"value", "[REDACTED]",
			)
		} else {
			c.diag.log("load: field resolved",
				"key", f.key,
				"source", src,
				"value", c.v.Get(f.key),
			)
		}
	}
}

func (c *Config[T]) determineSource(f fieldMeta) string {
	if c.setKeys[f.key] {
		return "set"
	}

	if c.opts.flags != nil && f.flag != "" {
		if pf := c.opts.flags.Lookup(f.flag); pf != nil && pf.Changed {
			return "flag"
		}
	}

	if f.env != "" {
		if _, ok := os.LookupEnv(f.env); ok {
			return "env"
		}
	}

	if c.filev != nil && c.filev.IsSet(f.key) {
		return "file"
	}

	if f.hasDefault {
		return "default"
	}

	return ""
}

func (c *Config[T]) hasKey(key string) bool {
	for _, f := range c.fields {
		if f.key == key {
			return true
		}
	}
	return false
}

func (c *Config[T]) writePath() string {
	return storePath(&c.opts)
}

func isZero(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return true
	}
	return rv.IsZero()
}
