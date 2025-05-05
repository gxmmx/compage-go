package core

// -----------------------------------------------------------------------------
// Unit kind types
// -----------------------------------------------------------------------------

type UnitKind string

const (
	UnitKindFunc UnitKind = "function"
	UnitKindPeer UnitKind = "clusterpeer"
	UnitKindData UnitKind = "datahandler"
	UnitKindRepo UnitKind = "repository"
	UnitKindServ UnitKind = "service"
	UnitKindPort UnitKind = "transport"
)

// -----------------------------------------------------------------------------
// Application State types
// -----------------------------------------------------------------------------

type AppReturnCode uint8

type appState uint8

const (
	appCreated appState = iota
	appInitialized
	appRunning
	appStopping
	appCompleted
)
