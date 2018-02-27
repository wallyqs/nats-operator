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

// Run starts the controller loop for a single NATS cluster.
func (ncc *NatsClusterController) Run(ctx context.Context) error {
	ncc.Debugf("Starting controller")
	// FIXME: Should include context internally in the client-go
	// requests to timeout and cancel the inflight requests.

	// Create headless service for the set of pods, then another
	// one for the clients.
	if err := ncc.createServices(ctx); err != nil {
		ncc.Errorf("Error during the creation of required services: %s", err)
		return err
	}

	// Create the initial set of pods in the cluster. If Pod
	// creation fails in this step then eventually reconcile
	// should try to do it.
	if err := ncc.createPods(ctx); err != nil {
		ncc.Errorf("Error during the creation of first set of pods: %s", err)
	}

	// Periodically check health of the NATS cluster against
	// the state in etcd and replace missing pods.
	for {
		select {
		case <-ctx.Done():
			// Signal that controller will stop.
			close(ncc.done)
			return ctx.Err()
		case <-time.After(10 * time.Second):
			// Check health of the NATS cluster against
			// the state in etcd.
			ncc.reconcile(ctx)
		}
	}

	return ctx.Err()
}

func (ncc *NatsClusterController) createServices(ctx context.Context) error {
	ncc.Debugf("Creating services")

	svc := NewNatsRoutesService(ncc.clusterName)
	_, err := ncc.kc.CoreV1().Services(ncc.namespace).Create(svc)
	if err != nil {
		ncc.Errorf("Warning: error creating service: %s", err)
	}

	svc = NewNatsClientsService(ncc.clusterName)
	_, err = ncc.kc.CoreV1().Services(ncc.namespace).Create(svc)
	if err != nil {
		ncc.Errorf("Warning: error creating service: %s", err)
	}

	return nil
}

func (ncc *NatsClusterController) createPods(ctx context.Context) error {
	size := int(*ncc.config.Spec.Size)
	ncc.Debugf("Creating %d pods", size)

	// Create the initial set of pods in the cluster.
	pods := make([]*k8sv1.Pod, size)
	routes := make([]string, size)
	for i := 0; i < size; i++ {
		// TODO: Allow stable names option as well?
		name := UniquePodName(ncc.clusterName)

		pod := &k8sv1.Pod{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app":         "nats",
					"natsCluster": ncc.clusterName,
					"natsVersion": ncc.config.Spec.Version,
				},
				Annotations: map[string]string{},
			},
			Spec: k8sv1.PodSpec{
				// Hostname+Subdomain required in order to properly
				// assemble the cluster using A records later on.
				Hostname:  name,
				Subdomain: RoutesServiceSubdomain(ncc.clusterName),
				Containers: []k8sv1.Container{
					NewNatsContainer(ncc.config.Spec.Version),
				},
				RestartPolicy: k8sv1.RestartPolicyNever,
			},
		}
		pods[i] = pod

		// The headless service will create A records as follows for the pods:
		//
		// $name.$clusterName.$namespace.svc.cluster.local
		//
		routes[i] = NatsClusterRouteURL(name, ncc.clusterName, ncc.namespace)
	}

	// Create the shared ConfigMap for the pods.
	configMap, err := NewNatsClusterConfigMap(ncc, pods, routes)
	if err != nil {
		return err
	}
	result, err := ncc.kc.CoreV1().ConfigMaps(ncc.namespace).Create(configMap)
	if err != nil {
		ncc.Errorf("error creating configmap: %s", err)
	} else {
		ncc.configMap = result
	}
	// TODO: Should it wait for the config map to be created?

	for _, pod := range pods {
		volumes := make([]k8sv1.Volume, 0)
		volumeMounts := make([]k8sv1.VolumeMount, 0)

		// ConfigMap: Volume declaration for the Pod and Container.
		volume := NewConfigMapVolume(ncc.clusterName)
		volumes = append(volumes, volume)
		volumeMount := NewConfigMapVolumeMount()
		volumeMounts = append(volumeMounts, volumeMount)

		// In case TLS was enabled as part of the NATS cluster
		// configuration then should include the configuration here.
		if ncc.config.Spec.TLS != nil {
			if ncc.config.Spec.TLS.ServerSecret != "" {
				volume = NewServerSecretVolume(ncc.config.Spec.TLS.ServerSecret)
				volumes = append(volumes, volume)

				volumeMount := NewServerSecretVolumeMount()
				volumeMounts = append(volumeMounts, volumeMount)
			}

			if ncc.config.Spec.TLS.RoutesSecret != "" {
				volume = NewRoutesSecretVolume(ncc.config.Spec.TLS.RoutesSecret)
				volumes = append(volumes, volume)

				volumeMount := NewRoutesSecretVolumeMount()
				volumeMounts = append(volumeMounts, volumeMount)
			}
		}

		// Update the volumes mounts list from the NATS container
		// then create the pod.
		pod.Spec.Volumes = volumes
		container := pod.Spec.Containers[0]
		container.VolumeMounts = volumeMounts
		pod.Spec.Containers[0] = container

		// Create the configured pod.
		result, err := ncc.kc.CoreV1().Pods(ncc.namespace).Create(pod)
		if err != nil {
			ncc.Errorf("Could not create pod: %s", err)

			// Skip creating this pod in case it failed.
			continue
		}
		ncc.Lock()
		ncc.pods[pod.Name] = result
		ncc.Unlock()

		ncc.Tracef("Created Pod '%s'", pod.Name)
	}

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
