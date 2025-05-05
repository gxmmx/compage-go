package core

import "sync"

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type registeredUnits struct {
	mu    sync.RWMutex
	ukeys []string
	units map[string]Unit
}

// -----------------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------------

func (ru *registeredUnits) set(key string, value Unit) {
	ru.mu.Lock()
	defer ru.mu.Unlock()

	if _, exists := ru.units[key]; !exists {
		ru.ukeys = append(ru.ukeys, key)
	}
	ru.units[key] = value
}

func (ru *registeredUnits) get(key string) (Unit, bool) {
	ru.mu.RLock()
	defer ru.mu.RUnlock()

	val, ok := ru.units[key]
	return val, ok
}

func (ru *registeredUnits) keys() []string {
	ru.mu.RLock()
	defer ru.mu.RUnlock()

	ukeysCp := make([]string, len(ru.ukeys))
	copy(ukeysCp, ru.ukeys)
	return ukeysCp
}

func (ru *registeredUnits) Values() []Unit {
	ru.mu.RLock()
	defer ru.mu.RUnlock()

	values := make([]Unit, 0, len(ru.ukeys))
	for _, k := range ru.ukeys {
		values = append(values, ru.units[k])
	}
	return values
}

func (ru *registeredUnits) Len() int {
	ru.mu.RLock()
	defer ru.mu.RUnlock()

	return len(ru.ukeys)
}

func (ru *registeredUnits) Has(key string) bool {
	ru.mu.RLock()
	defer ru.mu.RUnlock()

	_, exists := ru.units[key]
	return exists
}
