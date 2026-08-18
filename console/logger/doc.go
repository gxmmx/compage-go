// Package logger provides structured JSON logging with stable program, unit,
// and run identifiers. Logger is safe for concurrent use and New never fails:
// when a configured file cannot be opened, it keeps a working stderr fallback.
//
// PreLogger records startup messages before a Logger is available. It is for
// single-goroutine initialization only and becomes unusable after Flush.
package logger
