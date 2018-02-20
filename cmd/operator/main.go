package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	natsoperator "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/operator"
)

var (
	// Command Line Options Flags
	showHelp    bool
	showVersion bool
	debugMode   bool
	traceMode   bool
	namespace   string
	podname     string
)

func init() {
	fs := flag.NewFlagSet("nats-server-operator", flag.ExitOnError)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: nats-server-operator [options...]\n\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Help and version
	fs.BoolVar(&showHelp, "h", false, "Show help")
	fs.BoolVar(&showHelp, "help", false, "Show help")
	fs.BoolVar(&showVersion, "v", false, "Show version")
	fs.BoolVar(&showVersion, "version", false, "Show version")

	// Logging options
	fs.BoolVar(&debugMode, "debug", false, "Show debug logs")
	fs.BoolVar(&debugMode, "D", false, "Show debug logs")
	fs.BoolVar(&traceMode, "trace", false, "Show trace logs")
	fs.BoolVar(&traceMode, "V", false, "Show trace logs")

	// Kubernetes options
	fs.StringVar(&namespace, "ns", "default", "Kubernetes Pod Namespace")
	fs.StringVar(&namespace, "namespace", "default", "Kubernetes Pod Namespace")
	fs.StringVar(&podname, "podname", "nats-server-operator", "Kubernetes Pod Name")

	fs.Parse(os.Args[1:])

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
		natsoperator.KubernetesOptions(namespace, podname),
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

			// If main context already done, then just exit.
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
				// Gracefully shutdown the component,
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
