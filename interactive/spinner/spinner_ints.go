package spinner

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Spinner interface {
	// Stop stops the spinner and prints the final message if not empty.
	Stop(msg string)
}
