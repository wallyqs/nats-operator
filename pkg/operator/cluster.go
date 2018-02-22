// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"
	"time"

	natscrdv1alpha2 "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/apis/nats.io/v1alpha2"
)

// NatsClusterController manages the health and configuration
// from a NATS cluster that is being operated.
type NatsClusterController struct {
	// CRD configuration from the cluster.
	crd  *natscrdv1alpha2.NatsCluster
	quit func()
	done chan struct{}

	// namespace and name of the object.
	namespace string
	name      string

	// Logging options
	logger Logger
	debug  bool
	trace  bool
}

// Run...
func (ncc *NatsClusterController) Run(ctx context.Context) error {
	ncc.Debugf("Starting controller")

	// Reconcile loop.
	for {
		select {
		case <-ctx.Done():
			// Prepare to shutdown...
			close(ncc.done)
			return ctx.Err()
		case <-time.After(5 * time.Second):
			ncc.reconcile(ctx)
		}
	}

	return ctx.Err()
}

func (ncc *NatsClusterController) reconcile(ctx context.Context) error {
	ncc.Debugf("Reconciling cluster")

	// TODO: List the number of pods that match the selector from
	// this cluster.

	return ctx.Err()
}

// Stop prepares the asynchronous shutdown of the cluster.
func (ncc *NatsClusterController) Stop() error {
	ncc.Debugf("Stopping controller")

	// Signal operator that controller is done.
	ncc.quit()
	return nil
}
