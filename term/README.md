# term

`term` provides a small terminal-detection helper.

```go
if term.IsTerminal(os.Stdout) {
    fmt.Println("interactive output")
}
```

Only `*os.File` writers can be terminals; other `io.Writer` implementations
return `false`.
