// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"time"

	natscrdv1alpha2 "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/apis/nats.io/v1alpha2"
	natscrdclient "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/generated/versioned"
	k8scrdclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8sfields "k8s.io/apimachinery/pkg/fields"
	k8sclient "k8s.io/client-go/kubernetes"
	k8srestapi "k8s.io/client-go/rest"
	k8scache "k8s.io/client-go/tools/cache"
)

// ConfigureKubernetesRESTClients takes the configuration and
// prepares the rest clients that will be used to interact
// with the cluster objects.
func (op *Operator) ConfigureKubernetesRESTClients(cfg *k8srestapi.Config) error {
	// REST client to make actions on the basic k8s types.
	kc, err := k8sclient.NewForConfig(cfg)
	if err != nil {
		return err
	}

	// REST client to manage API Server extensions/CRDs.
	kcrdc, err := k8scrdclient.NewForConfig(cfg)
	if err != nil {
		return err
	}

	// REST client to monitor NATS cluster resources.
	ncrdc, err := natscrdclient.NewForConfig(cfg)
	if err != nil {
		return err
	}

	op.kc = kc
	op.kcrdc = kcrdc
	op.ncrdc = ncrdc
	return nil
}

// NewNatsClusterResourcesInformer takes an operator and a set of
// resource handlers and returns an indexer and a controller that are
// subscribed to changes to the state of a NATS cluster resource.
func NewNatsClusterResourcesInformer(
	op *Operator,
	resourceFuncs k8scache.ResourceEventHandlerFuncs,
	interval time.Duration,
) (k8scache.Indexer, k8scache.Controller) {
	listWatcher := k8scache.NewListWatchFromClient(
		// Auto generated client from pkg/apis/nats.io/v1alpha2/types.go
		op.ncrdc.NatsV1alpha2().RESTClient(),

		// "natsclusters"
		CRDObjectPluralName,

		// Namespace where the NATS clusters are created.
		// TODO: Consider cluster wide option here?
		op.ns,
		k8sfields.Everything(),
	)
	return k8scache.NewIndexerInformer(
		listWatcher,
		&natscrdv1alpha2.NatsCluster{},

		// How often it will poll for the state
		// of the resources.
		interval,

		// Handlers
		resourceFuncs,
		k8scache.Indexers{},
	)
}
