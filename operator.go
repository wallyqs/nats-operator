// natsoperator is a Kubernetes Operator for NATS.
package natsoperator

import (
	"context"
)

// Operator manages NATS Clusters running in Kubernetes.
type Operator struct {
	// Start/Stop cancellation.
	ctx  context.Context
	quit func()

	// Logging Options.
	logger Logger
	debug  bool
	trace  bool
}

// Run starts the main loop.
func (op *Operator) Run(ctx context.Context) error {
	// Prepare setting up the CRD in in case it does not exist already.

	// Set up cancellation context for the main loop.
	ctx, cancelFn := context.WithCancel(ctx)
	op.quit = func() {
		cancelFn()
	}

	op.Noticef("Starting NATS Server Kubernetes Operator v%s", Version)
	for {
		// TODO: Implement events polling
		select {
		case <-ctx.Done():
			op.Noticef("Bye.")
			return ctx.Err()
		}
	}
}

// Shutdown gracefully shuts down the server.
func (op *Operator) Shutdown() {
	op.Noticef("Shutting down...")

	// Done.
	op.quit()
}

// Option is configuration option for the operator.
type Option func(*Operator) error

// NewOperator takes a variadic set of options and
// returns a configured operator.
func NewOperator(options ...Option) (*Operator, error) {
	op := &Operator{}

	// Customizations
	for _, opt := range options {
		if err := opt(op); err != nil {
			return nil, err
		}
	}

	if op.logger == nil {
		op.logger = &emptyLogger{}
	}

	return op, nil
}

// LoggingOptions parameterizes logging options for the operator.
func LoggingOptions(logger Logger, debug, trace bool) Option {
	return func(o *Operator) error {
		o.logger = logger
		o.debug = debug
		o.trace = trace
		return nil
	}
}
