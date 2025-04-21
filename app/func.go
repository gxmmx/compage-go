package app

import (
	"os"
	"os/signal"
	"syscall"
)

// Creates and listens for OS signals
func appSigChan() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	return sigChan
}
