package natsoperator

const (
	// Version is the version of the NATS server operator.
	Version = "0.1.0"

	// API Group used to register the CRD from the operator.
	APIGroup = "messaging.nats.io"

	// APIVersion is the version of the API that the operator supports.
	APIVersion = "v1alpha2"

	// KubeConfigEnvVar is the environment variable which if present
	// will make the operator it to authenticate to the cluster.
	KubeConfigEnvVar = "KUBERNETES_CONFIG_FILE"

	// CRDObjectName is the name used to refer to a single NATS cluster.
	CRDObjectName = "natscluster"

	// CRDObjectName is the name used to refer to a collection of NATS clusters.
	CRDObjectPluralName = "natsclusters"

	// CRDObjectKindName is the name used for the CRD in the requests payload.
	CRDObjectKindName = "NatsCluster"

	// CRDObjectFullName
	// e.g. natsclusters.messaging.nats.io
	CRDObjectFullName = CRDObjectPluralName + "." + APIGroup

	// Ports
	ClientPort     = int32(4222)
	ClusterPort    = int32(6222)
	MonitoringPort = int32(8222)

	// ContainerName is the name of the NATS server container.
	ContainerName = "nats-server"
)
