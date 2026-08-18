package config

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/gxmmx/compage-go/errx"
)

type options struct {
	file, configEnv, envPrefix string
	fileSet, configEnvSet      bool
	flagSet                    bool
	flags                      FlagSource
	validators                 map[string]FieldValidator
	initial                    []initialValue
}

type initialValue struct {
	key   string
	value any
}

// Option configures a Config. Options are validated during Load.
type Option func(*options) error

func WithFile(path string) Option {
	return func(o *options) error {
		if o.fileSet {
			return fmt.Errorf("WithFile specified more than once")
		}
		if path == "" {
			return fmt.Errorf("file path is required")
		}
		o.file = path
		o.fileSet = true
		return nil
	}
}

func WithConfigEnv(name string) Option {
	return func(o *options) error {
		if o.configEnvSet {
			return fmt.Errorf("WithConfigEnv specified more than once")
		}
		if name == "" {
			return fmt.Errorf("config environment variable is required")
		}
		o.configEnv = name
		o.configEnvSet = true
		return nil
	}
}

func WithEnvPrefix(prefix string) Option {
	return func(o *options) error { o.envPrefix = prefix; return nil }
}
func WithFlagSource(source FlagSource) Option {
	return func(o *options) error {
		if isNil(source) {
			return fmt.Errorf("flag source is required")
		}
		if o.flagSet {
			return fmt.Errorf("WithFlagSource specified more than once")
		}
		o.flags = source
		o.flagSet = true
		return nil
	}
}
func WithValidator(name string, fn FieldValidator) Option {
	return func(o *options) error {
		if name == "" || fn == nil {
			return fmt.Errorf("validator name and function are required")
		}
		if o.validators == nil {
			o.validators = map[string]FieldValidator{}
		}
		if _, ok := o.validators[name]; ok {
			return fmt.Errorf("duplicate validator %q", name)
		}
		o.validators[name] = fn
		return nil
	}
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	return (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface || v.Kind() == reflect.Map || v.Kind() == reflect.Slice || v.Kind() == reflect.Func) && v.IsNil()
}
func WithInitial(key string, value any) Option {
	return func(o *options) error {
		if key == "" {
			return fmt.Errorf("initial key is required")
		}
		for _, v := range o.initial {
			if v.key == key {
				return fmt.Errorf("duplicate initial key %q", key)
			}
		}
		o.initial = append(o.initial, initialValue{key, value})
		return nil
	}
}

func (c *Config[T]) options() (options, error) {
	o := options{validators: map[string]FieldValidator{}}
	for _, option := range c.opts {
		if option == nil {
			return o, configErr("nil option", errx.Invalid, "", "", "", nil, false)
		}
		if err := option(&o); err != nil {
			return o, configErr("invalid option", errx.Invalid, "", "", "", err, false)
		}
	}
	if o.configEnv != "" && strings.Contains(o.configEnv, "=") {
		return o, configErr("invalid config environment variable", errx.Invalid, "", "", "", nil, false)
	}
	if o.envPrefix != "" {
		if strings.HasPrefix(o.envPrefix, "_") || strings.HasSuffix(o.envPrefix, "_") {
			return o, configErr("invalid environment prefix", errx.Invalid, "", "", "", nil, false)
		}
		for _, r := range o.envPrefix {
			if !(r == '_' || unicode.IsUpper(r) || unicode.IsDigit(r)) {
				return o, configErr("invalid environment prefix", errx.Invalid, "", "", "", nil, false)
			}
		}
	}
	return o, nil
}
