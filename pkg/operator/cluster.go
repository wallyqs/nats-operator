// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"
	"fmt"
	"sync"
	"time"

	natscrdv1alpha2 "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/apis/nats.io/v1alpha2"
	natsconf "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/conf"
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
		return err
	}

	// Create the initial set of pods in the cluster. If Pod
	// creation fails in this step then eventually reconcile
	// should try to do it.
	if err := ncc.createPods(ctx); err != nil {
		// return err
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

	svc, err := NewNatsRoutesService(ncc.clusterName)
	if err != nil {
		return err
	}
	_, err = ncc.kc.CoreV1().Services(ncc.namespace).Create(svc)
	if err != nil {
		ncc.Errorf("Warning: error creating service: %s", err)
	}

	svc, err = NewNatsClientsService(ncc.clusterName)
	if err != nil {
		return err
	}
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
		// Generate a stable unique name for each one of the pods.
		name := GeneratePodName(ncc.clusterName)

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
				Subdomain: fmt.Sprintf("%s-routes", ncc.clusterName),
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
		routes[i] = fmt.Sprintf("%s.%s-routes.%s.svc",
			name, ncc.clusterName, ncc.namespace)
	}

	// Create the shared configuration
	sconfig := &natsconf.ServerConfig{
		Port:     int(ClientPort),
		HTTPPort: int(MonitoringPort),
		Debug:    true,
		Trace:    true,
		Cluster: &natsconf.ClusterConfig{
			Port:   int(ClusterPort),
			Routes: routes,
		},
	}
	rawConfig, err := natsconf.Marshal(sconfig)
	if err != nil {
		return err
	}

	// Create the config map which will be the mounted file.
	configMap := &k8sv1.ConfigMap{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: ncc.clusterName,
		},
		Data: map[string]string{
			"nats.conf": string(rawConfig),
		},
	}

	if result, err := ncc.kc.CoreV1().ConfigMaps(ncc.namespace).Create(configMap); err != nil {
		ncc.Errorf("Could not create config map: %s", err)
	} else {
		ncc.configMap = result
	}

	// TODO: Should it wait for the config map to be created as well?
	for _, pod := range pods {
		// Check if TLS was enabled in the pod
		ncc.Debugf("TLS was enabled: %+v", ncc.config.Spec.TLS)

		// For the Pod
		volumeName := "config"
		configVolume := k8sv1.Volume{
			Name: volumeName,
			VolumeSource: k8sv1.VolumeSource{
				ConfigMap: &k8sv1.ConfigMapVolumeSource{
					LocalObjectReference: k8sv1.LocalObjectReference{
						Name: ncc.clusterName,
					},
				},
			},
		}

		// For the Container
		configVolumeMount := k8sv1.VolumeMount{
			Name:      volumeName,
			MountPath: ConfigMapMountPath,
		}

		// TODO: In case TLS was enabled as part of the NATS cluster configuration
		// then should include the configuration here.
		volumeMounts := []k8sv1.VolumeMount{configVolumeMount}

		// Include the ConfigMap volume for each one of the pods.
		pod.Spec.Volumes = []k8sv1.Volume{
			configVolume,
		}

		// Include the volume as part of the mount list for the NATS container.
		container := pod.Spec.Containers[0]
		container.VolumeMounts = volumeMounts
		pod.Spec.Containers[0] = container

		result, err := ncc.kc.CoreV1().Pods(ncc.namespace).Create(pod)
		if err != nil {
			ncc.Errorf("Could not create pod: %s", err)
		}
		ncc.Lock()
		ncc.pods[pod.Name] = result
		ncc.Unlock()

		ncc.Noticef("Created Pod: %+v", result)
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
