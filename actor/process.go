package actor

type Envelope struct {
	Msg    any
	Sender *PID
}

type Processer interface {
	Start()
	PID() *PID
	Send(*PID, any, *PID)
	Invoke([]Envelope)
	Shutdown()
}
