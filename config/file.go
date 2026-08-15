package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gxmmx/compage-go/errx"
)

func loadFile(o options) (string, bool, map[string]any, error) {
	p := o.file
	if o.configEnv != "" {
		if v, ok := os.LookupEnv(o.configEnv); ok && v != "" {
			p = v
		}
	}
	if p == "" {
		return "", false, map[string]any{}, nil
	}
	p = os.ExpandEnv(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, nil, configErr("home directory", errx.Internal, "", "", "", err, true)
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	if p == "" || strings.HasPrefix(p, "~") {
		return "", false, nil, fileErr("invalid file path", errx.Invalid, p, nil, false)
	}
	p = filepath.Clean(p)
	ext := strings.ToLower(filepath.Ext(p))
	adapter, supported := formatForExtension(ext)
	if !supported {
		return "", false, nil, fileErr("unsupported file format", errx.Invalid, p, nil, false)
	}
	if _, statErr := os.Lstat(p); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return p, false, map[string]any{}, nil
		}
		return "", false, nil, fileErr("read file", errx.Internal, p, statErr, true)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false, nil, fileErr("read file", errx.Internal, p, err, true)
	}
	root, err := adapter.Decode(b)
	if err != nil {
		return "", false, nil, fileErr("parse file", errx.Invalid, p, err, false)
	}
	flat := map[string]any{}
	if err := flatten("", root, flat); err != nil {
		return "", false, nil, fileErr("decode file", errx.Invalid, p, err, false)
	}
	return p, true, flat, nil
}

// Save writes a fresh, source-aware document to the selected path.
func (c *Config[T]) Save() error {
	c.opMu.Lock()
	c.mu.RLock()
	if !c.loaded {
		c.mu.RUnlock()
		c.opMu.Unlock()
		return configErr("not loaded", errx.Conflict, "", "", "", nil, false)
	}
	st := c.st
	st.values = cloneValue(st.values)
	c.mu.RUnlock()
	o, err := c.options()
	if err != nil {
		c.opMu.Unlock()
		return err
	}
	r, err := makeRegistry[T](o)
	c.opMu.Unlock()
	if err != nil {
		return err
	}
	if st.path == "" {
		return configErr("no config file", errx.Conflict, "", "", "", nil, false)
	}
	root := map[string]any{}
	sensitive := false
	v := reflectValue(st.values)
	for _, f := range r.fields {
		origin := st.origins[f.key]
		if !f.save || !(origin.Source == SourceDefault || origin.Source == SourceFile || origin.Source == SourceSet) || (f.sensitive && origin.Source != SourceFile && origin.Source != SourceSet) {
			continue
		}
		if f.sensitive {
			sensitive = true
		}
		putNested(root, strings.Split(f.key, "."), reflectValueAt(v, f.index).Interface())
	}
	b, err := encodeFile(st.path, root)
	if err != nil {
		return fileErr("encode file", errx.Internal, st.path, err, false)
	}
	if info, e := os.Lstat(st.path); e == nil && !info.Mode().IsRegular() {
		return fileErr("unsafe save target", errx.Conflict, st.path, nil, false)
	} else if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return fileErr("stat save target", errx.Internal, st.path, e, true)
	}
	mode := os.FileMode(0o644)
	if sensitive {
		mode = 0o600
	}
	tmp, err := os.CreateTemp(filepath.Dir(st.path), ".config-*")
	if err != nil {
		return fileErr("create temporary file", errx.Internal, st.path, err, true)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(mode); err == nil {
		_, err = tmp.Write(b)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, st.path)
	}
	if err != nil {
		return fileErr("save file", errx.Internal, st.path, err, true)
	}
	// Directory sync is unavailable on some platforms; the file replacement is still
	// atomic there, with the documented weaker crash-durability guarantee.
	if directory, openErr := os.Open(filepath.Dir(st.path)); openErr == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func encodeFile(path string, root map[string]any) ([]byte, error) {
	adapter, ok := formatForExtension(filepath.Ext(path))
	if !ok {
		return nil, fmt.Errorf("unsupported extension")
	}
	return adapter.Encode(root)
}
func putNested(root map[string]any, path []string, value any) {
	m := root
	for _, p := range path[:len(path)-1] {
		n, ok := m[p].(map[string]any)
		if !ok {
			n = map[string]any{}
			m[p] = n
		}
		m = n
	}
	m[path[len(path)-1]] = value
}
