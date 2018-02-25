// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"
	"sync"
	"time"

	natscrdv1alpha2 "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/apis/nats.io/v1alpha2"
	k8sv1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	namespace   string
	clusterName string

	// kc is the Kubernetes client for making
	// requests to the Kubernetes API server.
	kc k8sclient.Interface

	// logging options from the controller.
	logger Logger
	debug  bool
	trace  bool

	// pods being managed by controller,
	// key is the name of the pod.
	pods map[string]*k8sv1.Pod

	// configMap is the shared config map holding the configuration
	// from the cluster.
	configMap *k8sv1.ConfigMap
}

// Run starts the controller loop to
func (ncc *NatsClusterController) Run(ctx context.Context) error {
	ncc.Debugf("Starting controller")

	// Create the initial set of pods in the cluster. If Pod
	// creation fails in this step then eventually reconcile
	// should try to do it.
	err := ncc.createPods(ctx)
	if err != nil {
		return err
	}

	// Reconcile loop.
	for {
		select {
		case <-ctx.Done():
			// Signal that controller will stop.
			close(ncc.done)
			return ctx.Err()
		case <-time.After(5 * time.Second):
			// Check against the state in etcd.
			ncc.reconcile(ctx)
		}
	}

	return ctx.Err()
}

func (ncc *NatsClusterController) createPods(ctx context.Context) error {
	ncc.Debugf("Creating pods for NATS cluster")

	size := int(*ncc.config.Spec.Size)
	pods := make([]*k8sv1.Pod, size)
	for i := 0; i < size; i++ {
		// Generate a stable unique name for each one of the pods.
		name := GeneratePodName(ncc.clusterName)

		labels := map[string]string{
			LabelAppKey:            LabelAppValue,
			LabelClusterNameKey:    ncc.clusterName,
			LabelClusterVersionKey: ncc.config.Spec.Version,
		}
		pod := &k8sv1.Pod{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:        name,
				Labels:      labels,
				Annotations: map[string]string{},
			},
			Spec: k8sv1.PodSpec{
				// Hosname+Subdomain required in order to properly
				// assemble the cluster using A records later on.
				Hostname:  name,
				Subdomain: ncc.clusterName,
				Containers: []k8sv1.Container{
					DefaultNatsContainer(ncc.config.Spec.Version),
				},
				RestartPolicy: k8sv1.RestartPolicyNever,
				// Volumes:       volumes,
			},
		}
		pods[i] = pod
	}

	// FIXME: Should include context internally too timeout and cancel
	// the requests...
	// routes := ""
	// for _, pod := range pods {
	// 	// Include volumes
	// 	// Include the ConfigMap volume for each one of the pods.
	// }
	configMap := &k8sv1.ConfigMap{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: ncc.clusterName,
		},
		Data: map[string]string{
			"nats.conf": `
"http_port": 8222
`,
		},
	}
	result, err := ncc.kc.CoreV1().ConfigMaps(ncc.namespace).Create(configMap)
	if err != nil {
		ncc.Errorf("Could not create config map: %s", err)
	}
	ncc.configMap = result

	// result, err := ncc.kc.CoreV1().Pods(ncc.namespace).Create(pod)
	// if err != nil {
	// 	ncc.Errorf("Could not create pod: %s", err)
	// }
	// ncc.Lock()
	// ncc.pods[pod.Name] = result
	// ncc.Unlock()

	// ncc.Noticef("Created Pod: %+v", result)

	return nil
}

func (ncc *NatsClusterController) reconcile(ctx context.Context) error {
	ncc.Debugf("Reconciling cluster")

	// TODO: List the number of pods that match the selector from
	// this cluster, then create some more if they are missing.

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
