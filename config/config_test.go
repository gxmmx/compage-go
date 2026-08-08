package config

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

type flatConfig struct {
	LogLevel string `cfg:"log_level" flag:"log-level" env:"LOG_LEVEL" default:"info"`
	Name     string `cfg:"name"      flag:"name"      env:"NAME"      default:""`
	Port     int    `cfg:"port"      flag:"port"      env:"PORT"      default:"9090"`
	Debug    bool   `cfg:"debug"     flag:"debug"     env:"DEBUG"     default:"false"`
}

type nestedConfig struct {
	LogLevel string        `cfg:"log_level" env:"LOG_LEVEL" default:"info"`
	Network  networkConfig `cfg:"network"`
}

type networkConfig struct {
	Addr string `cfg:"addr" flag:"addr" env:"ADDR" default:"localhost"`
	Port int    `cfg:"port" flag:"port" env:"PORT" default:"8080"`
}

type requiredConfig struct {
	Name   string `cfg:"name"    default:"app"`
	APIKey string `cfg:"api_key" env:"API_KEY" required:"true"`
}

type sensitiveConfig struct {
	Name   string `cfg:"name"    save:"true" default:"myapp"`
	Secret string `cfg:"secret"  save:"true" sensitive:"true"`
}

type saveConfig struct {
	LogLevel string `cfg:"log_level" save:"true" flag:"log-level" env:"LOG_LEVEL" default:"info"`
	Name     string `cfg:"name"      save:"true" flag:"name"      env:"NAME"      default:""`
	Port     int    `cfg:"port"      save:"true" flag:"port"      env:"PORT"      default:"9090"`
	Debug    bool   `cfg:"debug"     save:"true" flag:"debug"     env:"DEBUG"     default:"false"`
}

type filelessConfig struct {
	LogLevel string `cfg:"log_level" flag:"log-level" env:"LOG_LEVEL" default:"info"`
	Name     string `cfg:"name"      flag:"name"      env:"NAME"      default:"app"`
	Verbose  bool   `cfg:"verbose"   flag:"verbose"   default:"false"`
}

type mixedSaveConfig struct {
	Persist   string `cfg:"persist" save:"true" default:"saved"`
	Transient string `cfg:"transient" default:"not-saved"`
}

type dupFlagConfig struct {
	Addr string `cfg:"addr" flag:"listen"`
	Port string `cfg:"port" flag:"listen"`
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load[flatConfig]()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := cfg.Values()
	assertEqual(t, "log_level", v.LogLevel, "info")
	assertEqual(t, "name", v.Name, "")
	assertEqual(t, "port", v.Port, 9090)
	assertEqual(t, "debug", v.Debug, false)
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `
log_level = "debug"
name = "agent-1"
port = 7070
debug = true
`)

	cfg, err := Load[flatConfig](
		WithPath(dir),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := cfg.Values()
	assertEqual(t, "log_level", v.LogLevel, "debug")
	assertEqual(t, "name", v.Name, "agent-1")
	assertEqual(t, "port", v.Port, 7070)
	assertEqual(t, "debug", v.Debug, true)
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `
log_level = "debug"
port = 7070
`)

	t.Setenv("MYAPP_LOG_LEVEL", "warn")
	t.Setenv("MYAPP_PORT", "3000")

	cfg, err := Load[flatConfig](
		WithPath(dir),
		WithName("config"),
		WithEnvPrefix("MYAPP"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := cfg.Values()
	assertEqual(t, "log_level", v.LogLevel, "warn")
	assertEqual(t, "port", v.Port, 3000)
}

func TestFlagOverridesEnvAndFile(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `
log_level = "debug"
port = 7070
`)

	t.Setenv("MYAPP_LOG_LEVEL", "warn")

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("log-level", "info", "")
	fs.Int("port", 9090, "")
	_ = fs.Parse([]string{"--log-level=error"})

	cfg, err := Load[flatConfig](
		WithPath(dir),
		WithName("config"),
		WithEnvPrefix("MYAPP"),
		WithFlags(fs),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := cfg.Values()
	assertEqual(t, "log_level", v.LogLevel, "error")
	// port flag not changed — env should win over file
	assertEqual(t, "port", v.Port, 7070)
}

func TestUnchangedFlagDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `port = 5555`)

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Int("port", 9090, "")

	cfg, err := Load[flatConfig](
		WithPath(dir),
		WithName("config"),
		WithFlags(fs),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	assertEqual(t, "port", cfg.Values().Port, 5555)
}

func TestNestedStruct(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `
log_level = "warn"

[network]
addr = "0.0.0.0"
port = 4040
`)

	cfg, err := Load[nestedConfig](
		WithPath(dir),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := cfg.Values()
	assertEqual(t, "log_level", v.LogLevel, "warn")
	assertEqual(t, "network.addr", v.Network.Addr, "0.0.0.0")
	assertEqual(t, "network.port", v.Network.Port, 4040)
}

func TestNestedEnvVars(t *testing.T) {
	t.Setenv("APP_NETWORK_ADDR", "10.0.0.1")
	t.Setenv("APP_NETWORK_PORT", "6060")

	cfg, err := Load[nestedConfig](WithEnvPrefix("APP"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := cfg.Values()
	assertEqual(t, "network.addr", v.Network.Addr, "10.0.0.1")
	assertEqual(t, "network.port", v.Network.Port, 6060)
}

func TestNestedFlags(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("addr", "localhost", "")
	fs.Int("port", 8080, "")
	_ = fs.Parse([]string{"--addr=192.168.1.1", "--port=9999"})

	cfg, err := Load[nestedConfig](WithFlags(fs))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := cfg.Values()
	assertEqual(t, "network.addr", v.Network.Addr, "192.168.1.1")
	assertEqual(t, "network.port", v.Network.Port, 9999)
}

func TestFileLessMode(t *testing.T) {
	cfg, err := Load[flatConfig]()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	assertEqual(t, "log_level", cfg.Values().LogLevel, "info")
	assertEqual(t, "path", cfg.Path(), "")
}

func TestMissingFileIsGraceful(t *testing.T) {
	cfg, err := Load[flatConfig](
		WithPath("/nonexistent/path"),
		WithName("missing"),
	)
	if err != nil {
		t.Fatalf("Load() should not fail on missing file: %v", err)
	}

	assertEqual(t, "log_level", cfg.Values().LogLevel, "info")
}

func TestMalformedFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `this is not valid toml [[[`)

	_, err := Load[flatConfig](
		WithPath(dir),
		WithName("config"),
	)
	if err == nil {
		t.Fatal("expected error for malformed file, got nil")
	}
}

func TestRequiredFieldMissing(t *testing.T) {
	_, err := Load[requiredConfig]()
	if err == nil {
		t.Fatal("expected ValidationError, got nil")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}

	if len(ve.Missing) != 1 || ve.Missing[0] != "api_key" {
		t.Errorf("Missing = %v, want [api_key]", ve.Missing)
	}
}

func TestRequiredFieldPresent(t *testing.T) {
	t.Setenv("API_KEY", "secret123")

	_, err := Load[requiredConfig](WithEnvPrefix(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDuplicateFlagDetection(t *testing.T) {
	_, err := Load[dupFlagConfig]()
	if err == nil {
		t.Fatal("expected ParseError for duplicate flags, got nil")
	}

	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestConfigEnvEmptyNoPaths(t *testing.T) {
	t.Setenv("EMPTY_CFG", "")

	cfg, err := Load[flatConfig](WithConfigEnv("EMPTY_CFG"))
	if err != nil {
		t.Fatalf("expected graceful load with no file tier, got error: %v", err)
	}

	if cfg.Values().LogLevel != "info" {
		t.Errorf("expected default log_level, got %q", cfg.Values().LogLevel)
	}
	if cfg.Path() != "" {
		t.Errorf("expected empty path, got %q", cfg.Path())
	}
}

func TestConfigEnvEmptyFallsToPaths(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `log_level = "from-path"`)

	t.Setenv("EMPTY_CFG", "")

	cfg, err := Load[flatConfig](
		WithConfigEnv("EMPTY_CFG"),
		WithPath(dir),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	assertEqual(t, "log_level", cfg.Values().LogLevel, "from-path")
}

func TestConfigEnvOverridesPaths(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	writeTOML(t, dirA, "config.toml", `log_level = "from-path"`)
	writeTOML(t, dirB, "custom.toml", `log_level = "from-env"`)

	envPath := filepath.Join(dirB, "custom.toml")
	t.Setenv("MY_CONFIG_FILE", envPath)

	cfg, err := Load[flatConfig](
		WithPath(dirA),
		WithName("config"),
		WithConfigEnv("MY_CONFIG_FILE"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	assertEqual(t, "log_level", cfg.Values().LogLevel, "from-env")
	assertEqual(t, "path", cfg.Path(), envPath)
}

func TestConfigEnvNoExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myconfig")
	if err := os.WriteFile(path, []byte("log_level = \"custom\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CFG_PATH", path)

	cfg, err := Load[flatConfig](
		WithConfigEnv("CFG_PATH"),
		WithType("toml"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	assertEqual(t, "log_level", cfg.Values().LogLevel, "custom")
}

func TestSourceTracking(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `
log_level = "debug"
name = "from-file"
port = 5000
`)

	t.Setenv("MYAPP_NAME", "from-env")

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Int("port", 9090, "")
	_ = fs.Parse([]string{"--port=1234"})

	cfg, err := Load[flatConfig](
		WithPath(dir),
		WithName("config"),
		WithEnvPrefix("MYAPP"),
		WithFlags(fs),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	assertEqual(t, "log_level source", cfg.Source("log_level"), "file")
	assertEqual(t, "name source", cfg.Source("name"), "env")
	assertEqual(t, "port source", cfg.Source("port"), "flag")
	assertEqual(t, "debug source", cfg.Source("debug"), "default")
}

func TestSet(t *testing.T) {
	cfg, err := Load[flatConfig]()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := cfg.Set("log_level", "error"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	assertEqual(t, "log_level", cfg.Values().LogLevel, "error")
	assertEqual(t, "source", cfg.Source("log_level"), "set")
}

func TestSetUnknownKey(t *testing.T) {
	cfg, err := Load[flatConfig]()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	err = cfg.Set("nonexistent", "value")
	if !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("expected ErrUnknownKey, got: %v", err)
	}
}

func TestSavePreseedPattern(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load[sensitiveConfig](
		WithPath(dir),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := cfg.Set("secret", "mysecretvalue"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	s := string(content)
	if !contains(s, "secret") || !contains(s, "mysecretvalue") {
		t.Errorf("saved file should contain set sensitive field, got:\n%s", s)
	}
	if !contains(s, "name") || !contains(s, "myapp") {
		t.Errorf("saved file should contain default fields, got:\n%s", s)
	}
}

func TestSaveSensitiveExcludedWhenDefault(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load[sensitiveConfig](
		WithPath(dir),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	s := string(content)
	if contains(s, "secret") {
		t.Errorf("saved file should NOT contain unset sensitive field, got:\n%s", s)
	}
}

func TestSaveExcludesEnvAndFlag(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `name = "original"`)

	t.Setenv("MYAPP_LOG_LEVEL", "warn")

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Int("port", 9090, "")
	_ = fs.Parse([]string{"--port=1111"})

	cfg, err := Load[saveConfig](
		WithPath(dir),
		WithName("config"),
		WithEnvPrefix("MYAPP"),
		WithFlags(fs),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	s := string(content)
	if contains(s, "warn") {
		t.Errorf("saved file should NOT contain env-sourced value 'warn', got:\n%s", s)
	}
	if contains(s, "1111") {
		t.Errorf("saved file should NOT contain flag-sourced value '1111', got:\n%s", s)
	}
	if !contains(s, "original") {
		t.Errorf("saved file should contain file-sourced value 'original', got:\n%s", s)
	}
}

func TestSaveNoConfigFile(t *testing.T) {
	cfg, err := Load[flatConfig]()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	err = cfg.Save()
	if !errors.Is(err, ErrNoConfigFile) {
		t.Fatalf("expected ErrNoConfigFile, got: %v", err)
	}
}

func TestPathSearchOrder(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	writeTOML(t, dirA, "config.toml", `log_level = "from-a"`)
	writeTOML(t, dirB, "config.toml", `log_level = "from-b"`)

	cfg, err := Load[flatConfig](
		WithPath(dirA),
		WithPath(dirB),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	assertEqual(t, "log_level", cfg.Values().LogLevel, "from-a")
}

func TestFlushDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	cfg, err := Load[flatConfig](
		WithPath("/nonexistent"),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	cfg.FlushDiagnostics(logger)

	output := buf.String()
	if output == "" {
		t.Fatal("expected diagnostic output, got empty")
	}
	if !contains(output, "registry: field discovered") {
		t.Errorf("expected registry messages in diagnostics, got:\n%s", output)
	}
	if !contains(output, "resolve:") {
		t.Errorf("expected resolve messages in diagnostics, got:\n%s", output)
	}
}

func TestFlushDiagnosticsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	cfg, err := Load[flatConfig]()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	cfg.FlushDiagnostics(logger)
	first := buf.String()

	buf.Reset()
	cfg.FlushDiagnostics(logger)
	second := buf.String()

	if second != "" {
		t.Errorf("second flush should produce no output, got:\n%s", second)
	}
	if first == "" {
		t.Error("first flush should produce output")
	}
}

func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load[sensitiveConfig](
		WithPath(dir),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := cfg.Set("secret", "value"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("permissions = %04o, want 0600", perm)
	}
}

func TestFilelessConfig(t *testing.T) {
	t.Setenv("MYAPP_LOG_LEVEL", "debug")

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("name", "default", "")
	_ = fs.Parse([]string{"--name=cli-arg"})

	cfg, err := Load[filelessConfig](
		WithEnvPrefix("MYAPP"),
		WithFlags(fs),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := cfg.Values()
	assertEqual(t, "log_level", v.LogLevel, "debug")
	assertEqual(t, "name", v.Name, "cli-arg")
	assertEqual(t, "verbose", v.Verbose, false)
	assertEqual(t, "path", cfg.Path(), "")

	assertEqual(t, "log_level source", cfg.Source("log_level"), "env")
	assertEqual(t, "name source", cfg.Source("name"), "flag")
	assertEqual(t, "verbose source", cfg.Source("verbose"), "default")
}

func TestFilelessConfigSaveReturnsError(t *testing.T) {
	cfg, err := Load[filelessConfig]()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	err = cfg.Save()
	if !errors.Is(err, ErrNoConfigFile) {
		t.Fatalf("expected ErrNoConfigFile for fileless config, got: %v", err)
	}
}

func TestSaveOnlyWritesSaveTaggedFields(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load[mixedSaveConfig](
		WithPath(dir),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	s := string(content)
	if !contains(s, "persist") || !contains(s, "saved") {
		t.Errorf("saved file should contain save-tagged field, got:\n%s", s)
	}
	if contains(s, "transient") {
		t.Errorf("saved file should NOT contain non-save field, got:\n%s", s)
	}
}

func TestFieldWithoutSaveStillReadFromFile(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `
persist = "from-file"
transient = "also-from-file"
`)

	cfg, err := Load[mixedSaveConfig](
		WithPath(dir),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := cfg.Values()
	assertEqual(t, "persist", v.Persist, "from-file")
	assertEqual(t, "transient", v.Transient, "also-from-file")
	assertEqual(t, "persist source", cfg.Source("persist"), "file")
	assertEqual(t, "transient source", cfg.Source("transient"), "file")
}

func TestSaveExcludesNonSaveFieldEvenIfFromFile(t *testing.T) {
	dir := t.TempDir()
	writeTOML(t, dir, "config.toml", `
persist = "keep"
transient = "drop-on-save"
`)

	cfg, err := Load[mixedSaveConfig](
		WithPath(dir),
		WithName("config"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	s := string(content)
	if !contains(s, "keep") {
		t.Errorf("saved file should contain save-tagged field, got:\n%s", s)
	}
	if contains(s, "drop-on-save") || contains(s, "transient") {
		t.Errorf("saved file should NOT contain non-save field, got:\n%s", s)
	}
}

// --- helpers ---

func writeTOML(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && bytes.Contains([]byte(s), []byte(substr))
}
