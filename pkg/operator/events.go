// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"
	"time"

	natscrdv1alpha2 "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/apis/nats.io/v1alpha2"
	k8sv1 "k8s.io/api/core/v1"
)

// processAdd
func (op *Operator) processAdd(ctx context.Context, o interface{}) {
	config := o.(*natscrdv1alpha2.NatsCluster)
	op.Tracef("Adding NATS Cluster: %+v", config)

	controller := &NatsClusterController{
		config:      config,
		logger:      op.logger,
		debug:       op.debug,
		trace:       op.trace,
		namespace:   config.Namespace,
		clusterName: config.Name,
		done:        make(chan struct{}),
		pods:        make(map[string]*k8sv1.Pod),
		kc:          op.kc,
	}

	op.Lock()
	if namespaceClusters, namespaceExists := op.clusters[config.Namespace]; namespaceExists {
		if _, clusterExists := namespaceClusters[config.Name]; clusterExists {
			op.Errorf("[%s/%s] Cluster already exists!", config.Namespace, config.Name)
			op.Unlock()
			return
		} else {
			// Create the cluster in that namespace
			namespaceClusters[config.Name] = controller
		}
	} else {
		op.clusters[config.Namespace] = map[string]*NatsClusterController{
			config.Name: controller,
		}
	}
	op.Unlock()

	// Run the controller branching from main context.
	ctx, cancelFn := context.WithCancel(ctx)
	controller.quit = func() {
		// Stop controller loop.
		cancelFn()

		// Wait for it to stop or give up in order to unblock
		// the operator from shutting down.
		select {
		case <-controller.done:
		case <-time.After(10 * time.Second):
			controller.Errorf("Cluster took too long to be stopped!")
		}

		// Signal operator that controller is done.
		op.wg.Done()
	}
	op.wg.Add(1)
	go controller.Run(ctx)
}

// processUpdate
func (op *Operator) processUpdate(ctx context.Context, o interface{}, n interface{}) {
	oldc := o.(*natscrdv1alpha2.NatsCluster)
	newc := n.(*natscrdv1alpha2.NatsCluster)
	op.Tracef("Updating NATS Cluster: Old: %+v || New: %+v", oldc, newc)

	// TODO: Confirm which fields should this handle, probably
	// cluster size would be updated in order to scale up/down.
	// TODO: Enable other options.
}

// processDelete
func (op *Operator) processDelete(ctx context.Context, o interface{}) {
	config := o.(*natscrdv1alpha2.NatsCluster)
	op.Tracef("Deleting NATS Cluster: %+v", config)

	// TODO: Should stop the controller and delete the managed pods.
}
