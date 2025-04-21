package appold

import (
	"os"
	"os/signal"
	"syscall"
)

// Creates and listens for OS signals
func appSignal() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	return sigChan
}
