package natsoperator

import (
	"fmt"

	natsconf "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/conf"
	k8sv1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sintstr "k8s.io/apimachinery/pkg/util/intstr"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
)

// UniquePodName generates a unique name for the Pod.
func UniquePodName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, k8srand.String(5))
}

// NatsContainer generates the default configuration
// for the NATS container.
func NewNatsContainer(version string) k8sv1.Container {
	return k8sv1.Container{
		Name:  ContainerName,
		Image: fmt.Sprintf("nats:%s", version),
		Command: []string{
			"/gnatsd",
			"-c",
			"/etc/nats-config/nats.conf",
		},
		Ports: []k8sv1.ContainerPort{
			{
				Name:          "client",
				ContainerPort: ClientPort,
				Protocol:      k8sv1.ProtocolTCP,
			},
			{
				Name:          "cluster",
				ContainerPort: ClusterPort,
				Protocol:      k8sv1.ProtocolTCP,
			},
			{
				Name:          "monitoring",
				ContainerPort: MonitoringPort,
				Protocol:      k8sv1.ProtocolTCP,
			},
		},
	}
}

func NewNatsRoutesService(clusterName string) *k8sv1.Service {
	ports := []k8sv1.ServicePort{
		{
			Name:       "cluster",
			Port:       ClusterPort,
			TargetPort: k8sintstr.FromInt(int(ClusterPort)),
			Protocol:   k8sv1.ProtocolTCP,
		},
	}

	return &k8sv1.Service{
		ObjectMeta: k8smetav1.ObjectMeta{
			// nats-routes.default.svc.cluster.local
			// TODO: Modify here
			Name: RoutesServiceSubdomain(clusterName),
			Labels: map[string]string{
				"app":         "nats",
				"natsCluster": clusterName,
			},
		},
		Spec: k8sv1.ServiceSpec{
			Ports: ports,
			PublishNotReadyAddresses: true,
			Selector: map[string]string{
				"app":         "nats",
				"natsCluster": clusterName,
			},
			ClusterIP: k8sv1.ClusterIPNone,
		},
	}
}

func NewNatsClientsService(clusterName string) *k8sv1.Service {
	ports := []k8sv1.ServicePort{
		{
			Name:       "client",
			Port:       ClientPort,
			TargetPort: k8sintstr.FromInt(int(ClientPort)),
			Protocol:   k8sv1.ProtocolTCP,
		},
	}

	return &k8sv1.Service{
		ObjectMeta: k8smetav1.ObjectMeta{
			// nats.default.svc.cluster.local
			Name: clusterName,
			Labels: map[string]string{
				"app":         "nats",
				"natsCluster": clusterName,
			},
		},
		Spec: k8sv1.ServiceSpec{
			Ports: ports,
			PublishNotReadyAddresses: true,
			Selector: map[string]string{
				"app":         "nats",
				"natsCluster": clusterName,
			},
			ClusterIP: "",
		},
	}
}

// NewNatsClusterConfigMap take the current cluster configuration and
// then returns the proper config map to be shared by the NATS cluster
// nodes. Warning: natsconf package is very hacky.
func NewNatsClusterConfigMap(
	ncc *NatsClusterController,
	pods []*k8sv1.Pod,
	routes []string,
) (*k8sv1.ConfigMap, error) {
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

	if ncc.config.Spec.TLS != nil {
		// FIXME: Allow customizing the path to each one of these.
		if ncc.config.Spec.TLS.ServerSecret != "" {
			sconfig.TLS = &natsconf.TLSConfig{
				CAFile:   ServerCertsMountPath + "/ca.pem",
				CertFile: ServerCertsMountPath + "/server.pem",
				KeyFile:  ServerCertsMountPath + "/server-key.pem",
			}
		}

		if ncc.config.Spec.TLS.RoutesSecret != "" {
			sconfig.Cluster.TLS = &natsconf.TLSConfig{
				CAFile:   RoutesCertsMountPath + "/ca.pem",
				CertFile: RoutesCertsMountPath + "/routes.pem",
				KeyFile:  RoutesCertsMountPath + "/routes-key.pem",
			}
		}
	}

	rawConfig, err := natsconf.Marshal(sconfig)
	if err != nil {
		return nil, err
	}

	// Create the config map which will be the mounted file.
	return &k8sv1.ConfigMap{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: ncc.clusterName,
		},
		Data: map[string]string{
			"nats.conf": string(rawConfig),
		},
	}, nil
}

// RoutesServiceSubdomain is the name of the of the service for
// the routes and subdomain used for the wildcard certificate
// for securing the cluster routes with TLS by using an A record
// per NATS pod.
func RoutesServiceSubdomain(clusterName string) string {
	return fmt.Sprintf("%s-routes", clusterName)
}

// NatsClusterRouteURL generates the URL for a NATS cluster routes.
// FIXME: Add Authentication credentials.
func NatsClusterRouteURL(name, clusterName, namespace string) string {
	return fmt.Sprintf("nats://%s.%s.%s.svc:%d",
		name, RoutesServiceSubdomain(clusterName), namespace, ClusterPort)
}
