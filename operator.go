// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"

	k8scrdclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8sclient "k8s.io/client-go/kubernetes"
	k8srestapi "k8s.io/client-go/rest"
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

	// Kubernetes API clients.
	kclient    k8sclient.Interface
	kcrdclient k8scrdclient.Interface
}

// Run starts the main loop.
func (op *Operator) Run(ctx context.Context) error {
	// Setup configuration for when operator runs within
	// Kubernetes and the API client for making requests, then
	// prepare setting up the CRD in in case it does not exist
	// already.
	cfg, err := k8srestapi.InClusterConfig()
	if err != nil {
		return err
	}
	client, err := k8sclient.NewForConfig(cfg)
	if err != nil {
		return err
	}
	op.kclient = client

	kcrdclient, err := k8scrdclient.NewForConfig(cfg)
	if err != nil {
		return err
	}
	op.kcrdclient = kcrdclient

	op.Tracef("Kubernetes Cluster Config: %+v", cfg)
	op.Tracef("Kubernetes Client: %+v", client)
	op.Tracef("Kubernetes Extensions Client: %+v", kcrdclient)

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
			op.Noticef("Bye!")
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

	// Apply customizations and error out in case
	// any of each are invalid.
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
