package natsoperator

const (
	// Version is the version of the NATS server operator.
	Version = "0.1.0"

	// API Group used to register the CRD from the operator.
	APIGroup = "alpha.nats.io"

	// APIVersion is the version of the API that the operator supports.
	APIVersion = "v1alpha2"

	// KubeConfigEnvVar is the environment variable which if present
	// will make the operator use the
	KubeConfigEnvVar = "KUBERNETES_CONFIG_FILE"
)
