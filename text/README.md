# text

Text sanitization and manipulation utilities.

## Slugify

Sanitizes input for use as an identifier. Replaces runs of non-alphanumeric
characters with a separator, applies case conversion, and trims edges.

```go
text.Slugify("Hello World!", "-", text.Lower)    // "hello-world"
text.Slugify("app.config-key", "_", text.Upper)  // "APP_CONFIG_KEY"
text.Slugify("My Config Key", ".", text.Lower)   // "my.config.key"
text.Slugify("foo--bar-.baz", "-", text.Lower)   // "foo-bar-baz"
```

## StripPrefix

Removes a prefix and any immediately following separator (`_`, `-`, `.`).

```go
text.StripPrefix("APP_DB_HOST", "APP")  // "DB_HOST"
text.StripPrefix("app-name", "app")     // "name"
text.StripPrefix("foo.bar", "foo")      // "bar"
```

## EnsurePrefix / EnsureSuffix

Idempotent prefix/suffix operations.

```go
text.EnsurePrefix("/path", "/")   // "/path"
text.EnsurePrefix("path", "/")    // "/path"

text.EnsureSuffix("file.go", ".go")  // "file.go"
text.EnsureSuffix("file", ".go")     // "file.go"
```

## Truncate

Shortens to max runes with a tail indicator. Rune-aware for multibyte text.

```go
text.Truncate("hello world", 8, "...")  // "hello..."
text.Truncate("short", 10, "...")       // "short"
```
