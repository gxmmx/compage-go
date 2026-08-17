# host

`host` reports process and platform facts without embedding application policy.

```go
platform := host.Platform()
user, err := host.User()
if err != nil { return err }

if user.IsRoot && platform.OS == host.Linux {
    // perform an explicitly authorized system operation
}
```

Use `host.Process()` for the current PID and canonical executable path, and
`host.WorkingDir()` for the current working directory.
