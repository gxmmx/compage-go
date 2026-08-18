package config

import (
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gxmmx/compage-go/errx"
)

// fieldMeta is the immutable reflected description of one supported leaf field.
type fieldMeta struct {
	key, field, env, flag, validate string
	index                           []int
	typ                             reflect.Type
	required, sensitive, save       bool
	def                             string
	hasDef                          bool
}

type registry struct {
	fields []fieldMeta
	byKey  map[string]fieldMeta
}

func makeRegistry[T any](o options) (registry, error) {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		return registry{}, configErr("schema must be a struct", errx.Invalid, "", "", "", nil, false)
	}
	r := registry{byKey: map[string]fieldMeta{}}
	var walk func(reflect.Type, []string, []int) error
	walk = func(current reflect.Type, prefix []string, base []int) error {
		for i := 0; i < current.NumField(); i++ {
			sf := current.Field(i)
			if sf.PkgPath != "" {
				continue
			}
			idx := append(append([]int(nil), base...), i)
			cfg, cfgSet := sf.Tag.Lookup("cfg")
			if cfg == "-" {
				if hasConfigTag(sf) && tagCount(sf) > 1 {
					return configErr("excluded field has config tags", errx.Invalid, "", sf.Name, "", nil, false)
				}
				continue
			}
			name := cfg
			if !cfgSet {
				name = snake(sf.Name)
			}
			if name == "" || strings.Contains(name, ".") {
				return configErr("invalid config key", errx.Invalid, "", sf.Name, "", nil, false)
			}
			ft := sf.Type
			if ft.Kind() == reflect.Struct && ft != reflect.TypeFor[time.Duration]() {
				if hasNonNamespaceTag(sf) {
					return configErr("unsupported nested field tag", errx.Invalid, strings.Join(append(prefix, name), "."), sf.Name, "", nil, false)
				}
				if sf.Anonymous {
					if cfgSet {
						return configErr("embedded field cannot have cfg tag", errx.Invalid, "", sf.Name, "", nil, false)
					}
					if err := walk(ft, prefix, idx); err != nil {
						return err
					}
				} else if err := walk(ft, append(prefix, name), idx); err != nil {
					return err
				}
				continue
			}
			if !supported(ft) {
				return configErr("unsupported field type", errx.Invalid, strings.Join(append(prefix, name), "."), sf.Name, "", nil, false)
			}
			key := strings.Join(append(prefix, name), ".")
			f := fieldMeta{key: key, field: sf.Name, index: idx, typ: ft}
			if d, ok := sf.Tag.Lookup("default"); ok {
				f.def, f.hasDef = d, true
			}
			if v, ok := sf.Tag.Lookup("required"); ok {
				if v != "true" {
					return configErr("invalid required tag", errx.Invalid, key, sf.Name, "", nil, false)
				}
				f.required = true
			}
			if f.required && f.hasDef {
				return configErr("required field has default", errx.Invalid, key, sf.Name, "", nil, false)
			}
			for _, pair := range []struct {
				name string
				dst  *bool
			}{{"sensitive", &f.sensitive}, {"save", &f.save}} {
				if v, ok := sf.Tag.Lookup(pair.name); ok {
					if v != "true" {
						return configErr("invalid "+pair.name+" tag", errx.Invalid, key, sf.Name, "", nil, false)
					}
					*pair.dst = true
				}
			}
			if v, ok := sf.Tag.Lookup("validate"); ok {
				if v == "" {
					return configErr("unknown validator", errx.Invalid, key, sf.Name, "", nil, false)
				}
				if _, ok := o.validators[v]; !ok {
					return configErr("unknown validator", errx.Invalid, key, sf.Name, "", nil, false)
				}
				f.validate = v
			}
			if v, ok := sf.Tag.Lookup("env"); ok {
				if v != "-" && v == "" {
					return configErr("invalid env tag", errx.Invalid, key, sf.Name, "", nil, false)
				}
				if v != "-" {
					f.env = v
				}
			} else {
				f.env = derivedEnv(o.envPrefix, key)
			}
			if v, ok := sf.Tag.Lookup("flag"); ok {
				if v == "" {
					return configErr("invalid flag tag", errx.Invalid, key, sf.Name, "", nil, false)
				}
				f.flag = v
			}
			if _, ok := r.byKey[key]; ok {
				return configErr("duplicate config key", errx.Invalid, key, sf.Name, "", nil, false)
			}
			r.byKey[key] = f
			r.fields = append(r.fields, f)
		}
		return nil
	}
	if err := walk(t, nil, nil); err != nil {
		return registry{}, err
	}
	sort.Slice(r.fields, func(i, j int) bool { return r.fields[i].key < r.fields[j].key })
	for i := 1; i < len(r.fields); i++ {
		if strings.HasPrefix(r.fields[i].key, r.fields[i-1].key+".") {
			return registry{}, configErr("scalar conflicts with namespace", errx.Invalid, r.fields[i].key, r.fields[i].field, "", nil, false)
		}
	}
	seenEnv, seenFlag := map[string]bool{}, map[string]bool{}
	for _, f := range r.fields {
		if f.env != "" {
			if seenEnv[f.env] {
				return registry{}, configErr("duplicate environment variable", errx.Invalid, f.key, f.field, "", nil, false)
			}
			seenEnv[f.env] = true
		}
		if f.flag != "" {
			if seenFlag[f.flag] {
				return registry{}, configErr("duplicate flag", errx.Invalid, f.key, f.field, "", nil, false)
			}
			seenFlag[f.flag] = true
		}
	}
	return r, nil
}

func hasNonNamespaceTag(f reflect.StructField) bool {
	for _, k := range []string{"env", "flag", "default", "required", "sensitive", "save", "validate"} {
		if _, ok := f.Tag.Lookup(k); ok {
			return true
		}
	}
	return false
}

func supported(t reflect.Type) bool {
	return t == reflect.TypeFor[string]() || t == reflect.TypeFor[bool]() || t == reflect.TypeFor[int]() || t == reflect.TypeFor[int64]() || t == reflect.TypeFor[uint]() || t == reflect.TypeFor[uint64]() || t == reflect.TypeFor[float64]() || t == reflect.TypeFor[time.Duration]() || t == reflect.TypeFor[[]string]()
}
func hasConfigTag(f reflect.StructField) bool {
	for _, k := range []string{"cfg", "env", "flag", "default", "required", "sensitive", "save", "validate"} {
		if _, ok := f.Tag.Lookup(k); ok {
			return true
		}
	}
	return false
}
func tagCount(f reflect.StructField) int {
	n := 0
	for _, k := range []string{"cfg", "env", "flag", "default", "required", "sensitive", "save", "validate"} {
		if _, ok := f.Tag.Lookup(k); ok {
			n++
		}
	}
	return n
}
func derivedEnv(prefix, key string) string {
	k := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	if prefix != "" {
		return prefix + "_" + k
	}
	return k
}
func snake(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		if unicode.IsUpper(r) && i > 0 && (unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]))) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
