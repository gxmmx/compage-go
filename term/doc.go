// Package term provides small helpers for detecting terminal-backed streams.
//
// For example:
//
//	if term.IsTerminal(os.Stdout) {
//		fmt.Println("interactive output")
//	}
//
// IsTerminal recognizes only *os.File values. Other io.Writer implementations
// return false.
package term
