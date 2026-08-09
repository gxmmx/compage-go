package text

import "testing"

func TestSlugify_DefaultCase(t *testing.T) {
	got := Slugify("Hello World!", "-")
	if got != "hello-world" {
		t.Errorf("got %q, want %q", got, "hello-world")
	}
}

func TestSlugify_Lower(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World!", "hello-world"},
		{"  leading spaces  ", "leading-spaces"},
		{"foo--bar-.baz", "foo-bar-baz"},
		{"Already-slug", "already-slug"},
		{"CamelCase", "camelcase"},
		{"with 123 numbers", "with-123-numbers"},
		{"---", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := Slugify(tt.input, "-", Lower)
		if got != tt.want {
			t.Errorf("Slugify(%q, -, Lower) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugify_Upper(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"app.config-key", "APP_CONFIG_KEY"},
		{"hello world", "HELLO_WORLD"},
		{"DB_HOST", "DB_HOST"},
		{"mixed--separators..here", "MIXED_SEPARATORS_HERE"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input, "_", Upper)
		if got != tt.want {
			t.Errorf("Slugify(%q, _, Upper) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugify_DotSeparator(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Config Key", "my.config.key"},
		{"app-name", "app.name"},
		{"UPPER_CASE", "upper.case"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input, ".", Lower)
		if got != tt.want {
			t.Errorf("Slugify(%q, ., Lower) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugify_Preserve(t *testing.T) {
	got := Slugify("Hello World", "-", Preserve)
	if got != "Hello-World" {
		t.Errorf("got %q, want %q", got, "Hello-World")
	}
}

func TestStripPrefix(t *testing.T) {
	tests := []struct {
		input  string
		prefix string
		want   string
	}{
		{"APP_DB_HOST", "APP", "DB_HOST"},
		{"app-name", "app", "name"},
		{"foo.bar", "foo", "bar"},
		{"APP_DB_HOST", "OTHER", "APP_DB_HOST"},
		{"prefix", "prefix", ""},
		{"prefix_", "prefix", ""},
		{"prefixvalue", "prefix", "value"},
	}
	for _, tt := range tests {
		got := StripPrefix(tt.input, tt.prefix)
		if got != tt.want {
			t.Errorf("StripPrefix(%q, %q) = %q, want %q", tt.input, tt.prefix, got, tt.want)
		}
	}
}

func TestEnsurePrefix(t *testing.T) {
	tests := []struct {
		s      string
		prefix string
		want   string
	}{
		{"/path", "/", "/path"},
		{"path", "/", "/path"},
		{"http://x", "http://", "http://x"},
		{"x", "http://", "http://x"},
	}
	for _, tt := range tests {
		got := EnsurePrefix(tt.s, tt.prefix)
		if got != tt.want {
			t.Errorf("EnsurePrefix(%q, %q) = %q, want %q", tt.s, tt.prefix, got, tt.want)
		}
	}
}

func TestEnsureSuffix(t *testing.T) {
	tests := []struct {
		s      string
		suffix string
		want   string
	}{
		{"file.go", ".go", "file.go"},
		{"file", ".go", "file.go"},
		{"path/", "/", "path/"},
		{"path", "/", "path/"},
	}
	for _, tt := range tests {
		got := EnsureSuffix(tt.s, tt.suffix)
		if got != tt.want {
			t.Errorf("EnsureSuffix(%q, %q) = %q, want %q", tt.s, tt.suffix, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		max  int
		tail string
		want string
	}{
		{"hello world", 8, "...", "hello..."},
		{"short", 10, "...", "short"},
		{"exact", 5, "...", "exact"},
		{"toolong", 3, "...", "..."},
		{"ab", 1, "...", "."},
		{"hello", 5, "", "hello"},
		{"hello!", 5, "", "hello"},
	}
	for _, tt := range tests {
		got := Truncate(tt.s, tt.max, tt.tail)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d, %q) = %q, want %q", tt.s, tt.max, tt.tail, got, tt.want)
		}
	}
}

func TestTruncate_Runes(t *testing.T) {
	got := Truncate("héllo wörld", 8, "…")
	want := "héllo w…"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
