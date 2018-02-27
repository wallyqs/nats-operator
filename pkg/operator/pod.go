package natsoperator

import (
	"fmt"

	k8sv1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sintstr "k8s.io/apimachinery/pkg/util/intstr"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
)

// GeneratePodName
func GeneratePodName(prefix string) string {
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

func NewNatsRoutesService(clusterName string) (*k8sv1.Service, error) {
	ports := []k8sv1.ServicePort{
		{
			Name:       "cluster",
			Port:       ClusterPort,
			TargetPort: k8sintstr.FromInt(int(ClusterPort)),
			Protocol:   k8sv1.ProtocolTCP,
		},
	}

	svc := &k8sv1.Service{
		ObjectMeta: k8smetav1.ObjectMeta{
			// nats-routes.default.svc.cluster.local
			Name: fmt.Sprintf("%s-routes", clusterName),
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

	return svc, nil
}

func NewNatsClientsService(clusterName string) (*k8sv1.Service, error) {
	ports := []k8sv1.ServicePort{
		{
			Name:       "client",
			Port:       ClientPort,
			TargetPort: k8sintstr.FromInt(int(ClientPort)),
			Protocol:   k8sv1.ProtocolTCP,
		},
	}

	svc := &k8sv1.Service{
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

	return svc, nil
}
