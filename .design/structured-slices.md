# Structured slices in config

## Status

Design only. This document specifies the implementation required for
configuration fields containing a list of structured values:

    type Item struct {
        Key   string `cfg:"key"`
        Value string `cfg:"value"`
    }

    type AppConfig struct {
        Items []Item `cfg:"items" env:"APP_ITEMS" flag:"items" save:"true" validate:"items"`
    }

The intended file representation is a native list of objects:

    [[items]]
    key = "first"
    value = "alpha"

    [[items]]
    key = "second"
    value = "beta"

The implementation must support the collection consistently across defaults,
initial values, files, environment variables, flags, Set, validation, and
saving. A feature that only works for file configuration is incomplete.

## Desired schema

Add one supported field category:

    []T where T is an exported struct containing supported scalar fields

The first implementation should support the existing scalar types:

- string
- bool
- int and int64
- uint and uint64
- float64
- time.Duration

Reject nested slices, maps, pointers, interfaces, recursive values, and
unsupported element types. Existing []string support remains separate.

Element fields must be exported. Their config names derive from the existing
snake-case rule or an element-level cfg tag. For example:

    type Item struct {
        Key   string `cfg:"key"`
        Value string `cfg:"value"`
    }

The parent slice owns source and persistence behavior. Element fields must
reject default, required, env, flag, save, and sensitive tags because dynamic
indexes cannot provide stable bindings for those sources.

The parent field may use the existing default, required, env, flag, save,
sensitive, and validate tags.

## Source precedence

The existing precedence remains the contract:

    default < initial < file < environment < flag < set

Every source must produce the same typed []T value before validation and
publication.

### Defaults

The default tag contains a JSON array of objects because defaults are stored as
strings:

    Items []Item `default:"[{\"key\":\"first\",\"value\":\"alpha\"}]"`

Required fields cannot also have defaults, following the existing rules.

### Initial values and Set

WithInitial("items", value) and Set("items", value) accept either:

- a typed []T; or
- a JSON string containing the array of objects.

Typed values must be defensively copied. Invalid values must fail the operation
rather than being stringified.

### Files

TOML, YAML, and JSON use native arrays of objects. The decoder must recursively
coerce each object according to the element schema and reject:

- a scalar in place of the collection;
- non-object array elements;
- unknown element keys;
- missing or incorrectly typed element values; and
- invalid duplicate keys.

The resulting value must be the same typed slice produced by every other
source.

### Environment variables

The collection environment variable contains one canonical JSON array:

    APP_ITEMS=[{"key":"first","value":"alpha"}]

JSON is required because delimiter formats cannot safely represent arbitrary
values. Shell quoting is the caller's responsibility. An empty list
is represented by []; an empty environment value is not silently treated as
an empty list.

### Flags

The flag binding uses the same JSON representation:

    --items '[{"key":"first","value":"alpha"}]'

FlagSource continues to provide one string value for the complete field. The
generic config package must not import pflag or invent dynamic flags for list
indexes. A CLI may expose this as a normal string flag through the existing
flag-source adapter.

The flag tag is active only when the caller supplies WithFlagSource.

## Validation

The parent validator receives the typed collection:

    func validateItems(_ config.FieldContext, value any) error {
        items, ok := value.([]Item)
        if !ok {
            return errors.New("items must be a list")
        }
        // Validate keys, values, duplicate keys, and policy rules.
        return nil
    }

Validation runs after decoding and before publication for every source,
including defaults, initial values, files, environment values, flags, and Set.

The generic package should preserve collection field context. If indexed paths
are supported for element errors, use items[1].key. Otherwise the
application validator may include the index in its message.

An application validator may enforce:

- non-empty keys;
- unique keys; and
- any application-specific constraints on values.

The config package treats element values as data. It must not interpret,
execute, normalize, or shell-parse them.

## Saving and redaction

When the winning source is saveable, encoders emit native arrays of objects:

- TOML array-of-table syntax;
- JSON arrays containing objects; and
- YAML arrays containing objects.

Existing persistence rules remain in force:

- default, file, and Set values may be saved when save:"true" allows it;
- initial, environment, and flag values remain transient;
- saves remain atomic; and
- sensitive structured fields are redacted as a whole in diagnostics.

Element-level sensitivity is not required in the first implementation.

## Reflection, cloning, and lifecycle

- Compile and retain an immutable element schema in the registry.
- Reject unsupported element fields and invalid element tags during schema
  construction.
- Deep-copy slices and element values before storing or returning snapshots.
- Preserve concurrent-reader snapshot semantics.
- Leave the previous snapshot unchanged when decoding or validation fails.
- Do not create files or directories as a side effect of this feature.

## Implementation areas

1. Extend registry metadata to recognize []struct fields and compile the element
   schema.
2. Validate element field names, supported types, and forbidden tags.
3. Extend coercion for typed values, JSON strings, and native object arrays.
4. Apply the new coercion path to default, initial, file, environment, flag,
   and Set sources.
5. Add recursive cloning for structured-slice snapshots and caller inputs.
6. Extend all file encoders to emit native object arrays.
7. Preserve field context, source provenance, error classification, and
   sensitive-value redaction.
8. Update existing callers and examples to the final structured-slice model.

## Required tests

- Registry accepts valid []Item schemas.
- Registry rejects unsupported element types and forbidden element tags.
- TOML array-of-tables loads correctly.
- YAML and JSON object arrays load correctly.
- Defaults and typed initial values load correctly.
- JSON environment values load correctly.
- JSON flag values load correctly through FlagSource.
- Typed and JSON Set values load correctly.
- Source precedence remains default, initial, file, env, flag, set.
- Validators run for every source and receive the typed slice.
- Invalid JSON, wrong shapes, unknown keys, and wrong element types fail.
- Saved output uses native object arrays in each supported format.
- Environment and flag winners remain transient on save.
- Returned and stored values do not alias caller or decoder storage.
- Sensitive structured values are redacted.
- Existing config behavior is updated consistently wherever callers change.

## Generic typed coercion

Every source must converge on the declared element type. For the example above,
all of these inputs produce the same `[]Item` value:

- a typed `[]Item` supplied through `WithInitial` or `Set`;
- a JSON string containing an array of objects supplied by a default,
  environment variable, flag, initial value, or `Set`;
- a TOML array of tables;
- a YAML sequence of mappings; or
- a JSON array of objects from a file.

Object fields are coerced according to the element schema and their declared
Go types. A JSON string, for example, is not retained as an untyped map or
`[]any`; it becomes a `[]Item` whose fields have the types declared by the
application. A typed value must be cloned before it enters the configuration
snapshot.
