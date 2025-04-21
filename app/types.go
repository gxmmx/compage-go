package app

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Application interface {
	AddConfig()
	AddConfigs()
	Initialize()
}

// -----------------------------------------------------------------------------
// Common types
// -----------------------------------------------------------------------------

type appState uint8

const (
	appCreated appState = iota
	appBootstrapped
	appInitialized
	appRunning
	appStopping
	appCompleted
)
