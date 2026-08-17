// Package printer provides human-facing, level-gated terminal output with
// semantic markers, indentation, tables, and optional color. Printer writes
// normal output to stdout and warnings and errors to stderr by default; options
// can swap those standard streams. Derived printers share a write lock and are
// safe to use concurrently.
//
// Prompter adds stdin-based interactive prompts. Callers should check that stdin
// is a terminal before prompting rather than attempting to read from a pipe.
package printer
