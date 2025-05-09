package pid

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

const separator = "/"

type PID struct {
	Address string
	ID      string
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func New(address, id string) *PID {

	p := &PID{
		Address: address,
		ID:      id,
	}
	return p
}

// -----------------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------------

func (pid *PID) String() string {
	return pid.Address + separator + pid.ID
}

func (pid *PID) Equals(other *PID) bool {
	return pid.Address == other.Address && pid.ID == other.ID
}

func (pid *PID) Child(id string) *PID {
	childID := pid.ID + separator + id
	return New(pid.Address, childID)
}
