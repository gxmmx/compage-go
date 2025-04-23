package app

// -----------------------------------------------------------------------------
// Common types
// -----------------------------------------------------------------------------

type appReturnCode uint8

type appState uint8

const (
	appCreated appState = iota
	appInitialized
	appRunning
	appStopping
	appCompleted
)
