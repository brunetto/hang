package lss

import (
	"os"
	"os/signal"
	"syscall"
)

type Logger interface {
	Printf(args ...interface{})
}

// WaitForShutdown waits the quit signal
func LogStartAndStop(processName string, logger Logger) {
	// Create signal channel
	c := make(chan os.Signal, 1)
	// Catch stop signals and send them to the channel
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	// Spin goroutine
	go func(c chan os.Signal, processName string, logger Logger) {
		// Waiting for exit signal on the channel
		<-c

		logger.Printf("%v: stopped by the user", processName)
		os.Exit(0)
	}(c, processName, logger)

	logger.Printf("%v: started", processName)
}
