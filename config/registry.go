package config

import (
	"fmt"
	"reflect"
	"strings"
)

type fieldMeta struct {
	structPath string
	key        string
	flag       string
	env        string
	envSuffix  string
	defaultVal string
	hasDefault bool
	required   bool
	sensitive  bool
	save       bool
}

func buildRegistry(t reflect.Type, prefix string, structPrefix string, envPrefix string, diag *diagBuffer) ([]fieldMeta, error) {
	var fields []fieldMeta

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		cfgTag := f.Tag.Get("cfg")

		if f.Type.Kind() == reflect.Struct && cfgTag != "" {
			nestedPrefix := prefix + cfgTag + "."
			nestedStructPrefix := structPrefix + f.Name + "."
			nestedEnvPrefix := envPrefix + strings.ToUpper(cfgTag) + "_"
			nested, err := buildRegistry(f.Type, nestedPrefix, nestedStructPrefix, nestedEnvPrefix, diag)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
			continue
		}

		if cfgTag == "" {
			continue
		}

		key := prefix + cfgTag
		sp := structPrefix + f.Name

		meta := fieldMeta{
			structPath: sp,
			key:        key,
			flag:       f.Tag.Get("flag"),
			envSuffix:  f.Tag.Get("env"),
			required:   f.Tag.Get("required") == "true",
			sensitive:  f.Tag.Get("sensitive") == "true",
			save:       f.Tag.Get("save") == "true",
		}

		if def, ok := f.Tag.Lookup("default"); ok {
			meta.defaultVal = def
			meta.hasDefault = true
		}

		if meta.envSuffix != "" {
			meta.env = envPrefix + meta.envSuffix
		}

		diag.log("registry: field discovered",
			"struct_path", sp,
			"key", key,
			"flag", meta.flag,
			"env", meta.env,
			"default", meta.defaultVal,
			"required", meta.required,
			"sensitive", meta.sensitive,
			"save", meta.save,
		)

		fields = append(fields, meta)
	}

	return fields, nil
}

func validateRegistry(fields []fieldMeta) error {
	seen := make(map[string]string)
	for _, f := range fields {
		if f.flag == "" {
			continue
		}
		if prev, ok := seen[f.flag]; ok {
			return &ParseError{
				Field: f.structPath,
				Key:   f.key,
				Err:   fmt.Errorf("duplicate flag name %q (also used by %s)", f.flag, prev),
			}
		}
		seen[f.flag] = f.structPath
	}
	return nil
}
