// Package style applies ANSI terminal styling when it is appropriate for an
// output destination. A Styler decides whether it is active at construction
// time from NO_COLOR, a caller-provided suppress flag, and terminal detection;
// an inactive or zero-value Styler returns text unchanged.
//
// Use Raw only when ANSI escape sequences are explicitly required, because it
// always emits them without terminal or NO_COLOR checks.
package style
