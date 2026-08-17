# Change: String-slice parsing

## Goal

Change the configuration package's textual representation of `[]string` from a
JSON array to the CSV-style input used by Cobra/pflag `StringSlice` values.
This makes one textual format work consistently for defaults, environment
variables, generic flag sources, and the `configpflag` adapter.

This is an intentional format change. Do not retain a JSON parsing fallback or
otherwise promise compatibility with the old JSON-array form.

## Target behavior

For every source that supplies a `[]string` as a string:

| Input | Result |
| --- | --- |
| `one,two` | `[]string{"one", "two"}` |
| `"one,two",three` | `[]string{"one,two", "three"}` |
| `"say ""hello""",three` | `[]string{"say \"hello\"", "three"}` |
| empty string | `[]string{}` |
| malformed CSV | configuration load fails with the existing invalid-value error |

The parser must also accept the bracketed representation returned by
`pflag.Value.String()` for `StringSlice`, for example `[one,two]` and
`["one,two",three]`. Remove exactly one matching outer `[` and `]` pair before
applying the CSV parser. This is an adapter representation, not an alternate
JSON format.

File-based configuration remains type-native: JSON, TOML, and YAML arrays are
decoded as arrays and are still validated as arrays of strings. `Set` and
`WithInitial` continue to accept a native `[]string`.

## Implementation plan

1. Introduce an internal `parseStringSlice(text string) ([]string, error)` in
   `config/decode.go`, or move it to a small dedicated internal file if that
   keeps the coercion logic clearer.

   - Import `encoding/csv` and `strings`.
   - Return an allocated empty slice for empty input, matching pflag's
     `StringSlice` behavior.
   - If the input begins with `[` and ends with `]`, remove that one outer pair.
     Do not trim or otherwise normalize the contents; CSV parsing must retain
     intentional leading and trailing whitespace.
   - Parse one CSV record with `csv.NewReader(strings.NewReader(text)).Read()`.
   - Reject malformed CSV and do not silently accept extra records. Configure
     or validate the reader so embedded newlines/additional records cannot
     become ignored input.

2. Replace the `json.Unmarshal` branch for `t == reflect.TypeFor[[]string]()`
   in `coerce` with `parseStringSlice`.

   - Remove the now-unused `encoding/json` import from `config/decode.go` only
     if no other use remains in that file.
   - Keep the existing native `[]string` copy path and the `[]any` file-array
     conversion path unchanged.
   - This is the only string-to-slice conversion path. In particular, do not
     detect or decode `[...]` as JSON.

3. Update the relevant comments to explicitly describe the changed contract.

   - Add a doc comment to `parseStringSlice` stating that textual string slices
     use pflag-compatible CSV syntax and that the optional brackets exist to
     consume pflag's `Value.String()` output.
   - Replace any comments near `coerce` that describe JSON parsing for string
     slices. The comments should not suggest JSON remains a supported textual
     input.
   - In `configpflag/source.go`, expand the adapter comment to state that
     `StringSlice` values are supported through their pflag string
     representation. No pflag-specific parsing should be added to the core
     package.

4. Update public documentation.

   - In `config/README.md`, replace the statement that `[]string` values use a
     JSON string array with the CSV/pflag-compatible contract.
   - Include examples for a default and an environment variable, for example
     `default:"one,two"` and `MYAPP_TAGS=one,two`.
   - Document CSV quoting for values containing commas and quotes.
   - Update any package comments or examples that use JSON strings for
     `[]string` defaults.

5. Add focused tests.

   In `config/decode_test.go`:

   - table-test plain comma-separated values, quoted commas, escaped quotes,
     the empty string, and malformed CSV;
   - assert `[one,two]` and `["one,two",three]` decode as pflag-compatible
     input;
   - replace existing JSON-array defaults with CSV defaults, and add a test
     confirming JSON-looking input is not treated as a JSON array.

   In a config load test:

   - set an environment variable to CSV input and assert both decoded value and
     `SourceEnv` provenance;
   - set a CSV default and ensure it is decoded through the same path.

   In `configpflag/source_test.go` (or an integration test using both
   packages):

   - register `flags.StringSlice("tags", nil, "")`;
   - set `tags` once with `one,two` and once again with a value containing a
     quoted comma;
   - load config through `configpflag.Source` and assert all parsed values,
     ordering, and `SourceFlag` provenance.

6. Run `go test ./...` and, if the project uses it in CI, the repository's
   normal formatting and static-check commands.

## Non-goals

- Do not change the `FlagSource` interface; its string value remains sufficient
  once coercion understands pflag's bracketed CSV output.
- Do not add a dependency on pflag or Cobra to `config`.
- Do not support a JSON-array compatibility mode for textual values.
- Do not change structured file-array decoding or native `[]string` values from
  `Set` and `WithInitial`.
