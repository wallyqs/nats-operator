// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

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

// KubernetesOptions customizes the namespace and pod name
// that the operator will use.
func KubernetesOptions(namespace, podname string) Option {
	return func(o *Operator) error {
		o.ns = namespace
		o.podname = podname
		return nil
	}
}
