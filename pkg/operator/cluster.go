// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"
	"fmt"
	"sync"
	"time"

	natscrdv1alpha2 "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/apis/nats.io/v1alpha2"
	k8sv1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
	k8sclient "k8s.io/client-go/kubernetes"
)

// NatsClusterController manages the health and configuration
// from a NATS cluster that is being operated.
type NatsClusterController struct {
	sync.Mutex

	// CRD configuration from the cluster.
	config *natscrdv1alpha2.NatsCluster
	quit   func()
	done   chan struct{}

	// namespace and name of the object.
	namespace string
	name      string

	// logging options
	logger Logger
	debug  bool
	trace  bool

	// pods being managed by controller,
	// key is the name of the pod.
	pods map[string]interface{}

	// kc k8scorev1client.CoreV1Interface
	kc k8sclient.Interface
}

// Run...
func (ncc *NatsClusterController) Run(ctx context.Context) error {
	ncc.Debugf("Starting controller")

	// Create the first pod in the cluster, if this fails
	// then eventually reconcile should try to do it.
	ncc.createPods(ctx)

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

func (ncc *NatsClusterController) createPods(ctx context.Context) error {
	ncc.Debugf("Creating Pods")

	container := NewNatsContainer(ncc.config.Spec.Version)

	// We will generate a stable unique name for each one of the pods.
	name := fmt.Sprintf("%s-%s", ncc.name, k8srand.String(5))

	pod := &k8sv1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: name,
			// Labels:      labels,
			Annotations: map[string]string{},
		},
		Spec: k8sv1.PodSpec{
			Hostname:      name,
			Subdomain:     ncc.name,
			Containers:    []k8sv1.Container{container},
			RestartPolicy: k8sv1.RestartPolicyNever,
			// Volumes:       volumes,
		},
	}

	// FIXME: Should include context internally too timeout and cancel
	// the requests...
	result, err := ncc.kc.CoreV1().Pods(ncc.namespace).Create(pod)
	if err != nil {
		return err
	}

	ncc.Lock()
	ncc.pods[name] = result
	ncc.Unlock()

	ncc.Noticef("Created Pod: %+v", result)

	return nil
}

func (ncc *NatsClusterController) reconcile(ctx context.Context) error {
	ncc.Debugf("Reconciling cluster")

	// TODO: List the number of pods that match the selector from
	// this cluster.

	return ctx.Err()
}

// Stop prepares the asynchronous shutdown of the controller,
// (it does not stop the pods being managed by the operator).
func (ncc *NatsClusterController) Stop() error {
	ncc.Debugf("Stopping controller")

	// Signal operator that controller is done.
	ncc.quit()
	return nil
}
