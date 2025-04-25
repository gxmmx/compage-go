package app

import "sync"

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type UnitSink interface {
	get() (any, bool)
	set(any)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Sink struct {
	mu    sync.RWMutex
	data  any
	isSet bool
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewSink() *Sink {
	return &Sink{
		mu:    sync.RWMutex{},
		data:  nil,
		isSet: false,
	}
}

// -----------------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------------

// Set stores a typed value safely
func (s *Sink) set(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = v
	s.isSet = true
}

// Get retrieves the typed value safely
func (s *Sink) get() (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data, s.isSet
}
