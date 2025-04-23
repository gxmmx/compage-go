package app

import "sync"

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type UnitSink interface {
	get()
	set()
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Sink[T any] struct {
	mu    sync.RWMutex
	data  T
	isSet bool
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewSink[T any]() *Sink[T] {
	return &Sink[T]{
		mu:    sync.RWMutex{},
		data:  *new(T),
		isSet: false,
	}
}

// -----------------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------------

// Set stores a typed value safely
func (s *Sink[T]) set(v T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = v
	s.isSet = true
}

// Get retrieves the typed value safely
func (s *Sink[T]) get() (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data, s.isSet
}
