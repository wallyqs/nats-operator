package natsoperator

import (
	"fmt"

	k8sv1 "k8s.io/api/core/v1"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
)

// GeneratePodName
func GeneratePodName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, k8srand.String(5))
}

// NatsContainer generates the default configuration
// for the NATS container.
func DefaultNatsContainer(version string) k8sv1.Container {
	return k8sv1.Container{
		Name:    ContainerName,
		Image:   fmt.Sprintf("nats:%s", version),
		Command: []string{"/gnatsd", "-c", "/etc/nats-config/nats.conf"},
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
