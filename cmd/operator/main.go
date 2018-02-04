package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	natsoperator "github.com/nats-io/nats-kubernetes/operators/nats-server"
)

// Command Line Options Flags
var (
	showHelp    bool
	showVersion bool
	debugMode   bool
	traceMode   bool
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: nats-server-operator [options...]\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Show help and version
	flag.BoolVar(&showHelp, "h", false, "Show help")
	flag.BoolVar(&showHelp, "help", false, "Show help")
	flag.BoolVar(&showVersion, "v", false, "Show version")
	flag.BoolVar(&showVersion, "version", false, "Show version")

	// Logging options
	flag.BoolVar(&debugMode, "debug", false, "Show debug logs")
	flag.BoolVar(&debugMode, "D", false, "Show debug logs")
	flag.BoolVar(&traceMode, "trace", false, "Show trace logs")
	flag.BoolVar(&traceMode, "V", false, "Show trace logs")
	flag.Parse()

	switch {
	case showHelp:
		flag.Usage()
		os.Exit(0)
	case showVersion:
		fmt.Fprintf(os.Stderr, "NATS Server Kubernetes Operator v%s\n", natsoperator.Version)
		os.Exit(0)
	}
}

func main() {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	logger = log.With(logger, "time", log.DefaultTimestampUTC)
	logger = log.With(logger, "pid", os.Getpid())

	op, err := natsoperator.NewOperator(
		natsoperator.LoggingOptions(logger, debugMode, traceMode),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
		os.Exit(1)
	}

	// Top level context which if canceled stops the
	// the main loop from the operator.
	ctx := context.Background()

	// Signal handling.
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

		for sig := range c {
			op.Debugf("Trapped '%v' signal", sig)

			// Check if we are done already first.
			select {
			case <-ctx.Done():
				return
			default:
			}

			switch sig {
			case syscall.SIGINT:
				op.Noticef("Exiting...")
				os.Exit(0)
				return
			case syscall.SIGTERM:
				// Gracefully shutdown the component.
				op.Shutdown()
			}
		}
	}()

	// Run until the context is canceled.
	err = op.Run(ctx)
	if err != nil && err != context.Canceled {
		op.Errorf(err.Error())
		os.Exit(1)
	}
}
