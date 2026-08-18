package config

type testConfig struct {
	Port  int      `default:"8080" env:"TEST_CONFIG_PORT" flag:"port" save:"true"`
	Name  string   `default:"agent" save:"true"`
	Token string   `env:"TEST_CONFIG_TOKEN" sensitive:"true" save:"true"`
	Tags  []string `default:"a" save:"true"`
}

type testFlags map[string]struct {
	value   string
	changed bool
}

func (f testFlags) Lookup(name string) (string, bool, bool) {
	v, ok := f[name]
	return v.value, v.changed, ok
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
