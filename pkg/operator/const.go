package natsoperator

import (
	k8sapiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
)

var (
	// DefaultNATSClusterCRD returns a representation of the CRD to register.
	DefaultNATSClusterCRD = &k8sapiextensionsv1beta1.CustomResourceDefinition{
		// ---
		// apiVersion: apiextensions.k8s.io/v1beta1
		// kind: CustomResourceDefinition
		// ...
		TypeMeta: k8smetav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1beta1",
			Kind:       "CustomResourceDefinition",
		},
		// metadata:
		//   # name must match the spec fields below, and be
		//   # in the form: <plural>.<group>
		//   name: natsclusters.messaging.nats.io
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: CRDObjectFullName,
		},
		Spec: k8sapiextensionsv1beta1.CustomResourceDefinitionSpec{
			// # group name to use for REST API: /apis/<group>/<version>
			// group: messaging.nats.io
			//
			// # version name to use for REST API: /apis/<group>/<version>
			// version: v1alpha2
			//
			// # either Namespaced or Cluster
			// scope: Namespaced
			Group:   APIGroup,
			Version: APIVersion,
p
			// FIXME: Eventually be able to support all cluster.
			Scope: k8sapiextensionsv1beta1.ResourceScope("Namespaced"),
			Names: k8sapiextensionsv1beta1.CustomResourceDefinitionNames{
				// # plural name to be used in the
				// # URL:
				// # /apis/<group>/<version>/<plural>
				// plural: natsclusters
				Plural: CRDObjectPluralName,

				// # singular name to be used as an
				// # alias on the CLI and for display
				// singular: natscluster
				Singular: CRDObjectName,

				// # kind is normally the CamelCased singular type.
				// kind: NatsCluster
				Kind: CRDObjectKindName,

				// # shortNames allow shorter string
				// # to match your resource on the CLI
				// - shortNames: ["nats"]
				ShortNames: []string{"nats"},
			},
		},
	}
)
