# errx

`errx` adds transport-neutral error classification while preserving causes for
`errors.Is` and `errors.As`.

```go
err := errx.New("loading account", errx.WithKind(errx.NotFound), errx.WithCause(cause))

if errx.IsKind(err, errx.NotFound) {
    // map to a not-found response, retry policy, or CLI exit code
}
if errors.Is(err, cause) {
    // underlying cause remains observable
}
```

Domain packages should expose their own contextual error types and implement
`errx.Classified`; use `errx.Error` for general-purpose wrappers.
