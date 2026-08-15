package config

import (
	"log/slog"
	"os"
	"sync"

	"github.com/gxmmx/compage-go/errx"
)

// Config owns the current validated snapshot.
type Config[T any] struct {
	opMu   sync.Mutex
	mu     sync.RWMutex
	opts   []Option
	loaded bool
	st     state[T]
	set    map[string]any
	logs   logBuffer
}

func New[T any](opts ...Option) *Config[T]           { return &Config[T]{opts: append([]Option(nil), opts...)} }
func Load[T any](opts ...Option) (*Config[T], error) { c := New[T](opts...); return c, c.Load() }

func (c *Config[T]) Load() (err error) {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	c.logs.add(newLogRecord(slog.LevelDebug, "config load started"))
	defer func() {
		if err != nil {
			r := newLogRecord(slog.LevelError, "config load failed")
			r.AddAttrs(slog.String("error", err.Error()))
			c.logs.add(r)
		}
	}()
	o, err := c.options()
	if err != nil {
		return err
	}
	r, err := makeRegistry[T](o)
	if err != nil {
		return err
	}
	registryRecord := newLogRecord(slog.LevelDebug, "config registry built")
	registryRecord.AddAttrs(slog.Int("fields", len(r.fields)))
	c.logs.add(registryRecord)
	c.mu.RLock()
	set, previouslyLoaded := cloneRaw(c.set), c.loaded
	c.mu.RUnlock()
	if !previouslyLoaded {
		for _, v := range o.initial {
			if _, ok := r.byKey[v.key]; !ok {
				return configErr("unknown initial key", errx.Invalid, v.key, "", "", nil, false)
			}
			set[v.key] = cloneRawValue(v.value)
		}
	}
	layers := map[Source]map[string]any{SourceDefault: {}, SourceFile: {}, SourceEnv: {}, SourceFlag: {}, SourceSet: set}
	for _, f := range r.fields {
		if f.hasDef {
			layers[SourceDefault][f.key] = f.def
		}
	}
	path, fileLoaded, fileLayer, err := loadFile(o)
	if err != nil {
		return err
	}
	fileRecord := newLogRecord(slog.LevelDebug, "config file evaluated")
	fileRecord.AddAttrs(slog.String("path", path), slog.Bool("loaded", fileLoaded))
	c.logs.add(fileRecord)
	for key := range fileLayer {
		if _, ok := r.byKey[key]; !ok {
			return configErr("unknown file key", errx.Invalid, key, "", "file", nil, false)
		}
	}
	if fileLoaded {
		layers[SourceFile] = fileLayer
	}
	for _, f := range r.fields {
		if f.env != "" {
			if v, ok := os.LookupEnv(f.env); ok {
				layers[SourceEnv][f.key] = v
			}
		}
		if f.flag != "" && o.flags != nil {
			v, changed, found := o.flags.Lookup(f.flag)
			if !found {
				return configErr("flag binding", errx.Conflict, f.key, f.field, "flag", nil, false)
			}
			if changed {
				layers[SourceFlag][f.key] = v
			}
		}
	}
	st, err := resolve[T](r, layers, path, fileLoaded, o.validators)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.set, c.st, c.loaded = set, st, true
	c.mu.Unlock()
	c.logs.add(newLogRecord(slog.LevelDebug, "config load completed"))
	return nil
}

func (c *Config[T]) Values() T { c.mu.RLock(); defer c.mu.RUnlock(); return cloneValue(c.st.values) }
func (c *Config[T]) Source(key string) (Source, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	o, ok := c.st.origins[key]
	return o.Source, ok
}
func (c *Config[T]) Origin(key string) (Origin, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	o, ok := c.st.origins[key]
	return o, ok
}
func (c *Config[T]) HasSource(key string, source Source) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.st.present[source][key]
}
func (c *Config[T]) FileLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loaded && c.st.fileLoaded
}
func (c *Config[T]) Path() string                 { c.mu.RLock(); defer c.mu.RUnlock(); return c.st.path }
func (c *Config[T]) FlushLog(logger *slog.Logger) { c.logs.flush(logger) }
func (c *Config[T]) Set(key string, value any) error {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	c.mu.RLock()
	if !c.loaded {
		c.mu.RUnlock()
		return configErr("not loaded", errx.Conflict, "", "", "", nil, false)
	}
	oldSet, oldState := cloneRaw(c.set), c.st
	c.mu.RUnlock()
	o, err := c.options()
	if err != nil {
		return err
	}
	r, err := makeRegistry[T](o)
	if err != nil {
		return err
	}
	if _, ok := r.byKey[key]; !ok {
		return configErr("unknown key", errx.Invalid, key, "", "", nil, false)
	}
	set := oldSet
	set[key] = cloneRawValue(value)
	layers := cloneLayers(oldState.layers)
	layers[SourceSet] = set
	st, err := resolve[T](r, layers, oldState.path, oldState.fileLoaded, o.validators)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.set, c.st = set, st
	c.mu.Unlock()
	return nil
}
func (c *Config[T]) Validate() error {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	c.mu.RLock()
	if !c.loaded {
		c.mu.RUnlock()
		return configErr("not loaded", errx.Conflict, "", "", "", nil, false)
	}
	st := c.st
	c.mu.RUnlock()
	o, err := c.options()
	if err != nil {
		return err
	}
	r, err := makeRegistry[T](o)
	if err != nil {
		return err
	}
	_, err = resolve[T](r, st.layers, st.path, st.fileLoaded, o.validators)
	return err
}
