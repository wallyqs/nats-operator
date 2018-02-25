// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"
	"os"
	"sync"

	// k8sapiv1 "k8s.io/api/core/v1"

	k8scrdclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8sfields "k8s.io/apimachinery/pkg/fields"
	k8sclient "k8s.io/client-go/kubernetes"
	k8srestapi "k8s.io/client-go/rest"
	k8scache "k8s.io/client-go/tools/cache"
	k8sclientcmd "k8s.io/client-go/tools/clientcmd"

	// k8swatch "k8s.io/apimachinery/pkg/watch"
	natscrdv1alpha2 "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/apis/nats.io/v1alpha2"
	natscrdclient "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/generated/versioned"
)

// Operator manages NATS Clusters running in Kubernetes.
type Operator struct {
	sync.Mutex
	wg sync.WaitGroup

	// Start/Stop cancellation.
	ctx  context.Context
	quit func()

	// Logging Options.
	logger Logger
	debug  bool
	trace  bool

	// Kubernetes API clients.
	kc    k8sclient.Interface
	kcrdc k8scrdclient.Interface

	// NATS operator client.
	ncrdc natscrdclient.Interface

	// Kubernetes Pod Namespace.
	ns string

	// Kubernetes Pod Name.
	// TODO: Not needed?
	podname string

	// clusters that the operator is managing.
	// ["namespace"]["name"]NatsCluster
	clusters map[string]map[string]*NatsClusterController
}

// Run starts the main loop.
func (op *Operator) Run(ctx context.Context) error {
	op.Noticef("Starting NATS Server Kubernetes Operator v%s", Version)

	// Setup configuration for when operator runs inside/outside
	// the cluster and the API client for making requests.
	// By default, consider that the operator runs in Kubernetes,
	// but allow to use a config file too for dev/testing purposes.
	var err error
	var cfg *k8srestapi.Config
	if kubeconfig := os.Getenv(KubeConfigEnvVar); kubeconfig != "" {
		cfg, err = k8sclientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = k8srestapi.InClusterConfig()
	}
	if err != nil {
		return err
	}

	kc, err := k8sclient.NewForConfig(cfg)
	if err != nil {
		return err
	}
	op.kc = kc

	kcrdc, err := k8scrdclient.NewForConfig(cfg)
	if err != nil {
		return err
	}
	op.kcrdc = kcrdc

	// Get config for the NATS clusters.
	ncrdc, err := natscrdclient.NewForConfig(cfg)
	if err != nil {
		return err
	}

	op.Debugf("Kubernetes Cluster Config: %+v", cfg)
	op.Debugf("Kubernetes Client: %+v", kc)
	op.Debugf("Kubernetes Extensions Client: %+v", kcrdc)

	// Set up cancellation context for the main loop.
	ctx, cancelFn := context.WithCancel(ctx)

	// Setup connection between the Operator and Kubernetes
	// and register CRD to make available API group in case
	// not done already.
	if err := op.RegisterCRD(ctx); err != nil {
		return err
	}

	// Subscribe to changes to NatsCluster resources.
	listWatcher := k8scache.NewListWatchFromClient(
		// Auto generated client from pkg/apis/nats.io/v1alpha2/types.go
		ncrdc.NatsV1alpha2().RESTClient(),

		// "natsclusters"
		CRDObjectPluralName,

		// Namespace where the NATS clusters are created.
		// TODO: Consider cluster wide option here?
		op.ns,
		k8sfields.Everything(),
	)
	_, informer := k8scache.NewIndexerInformer(listWatcher, &natscrdv1alpha2.NatsCluster{}, 0, k8scache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			op.processAdd(ctx, o)
		},
		UpdateFunc: func(o interface{}, n interface{}) {
			op.processUpdate(ctx, o, n)
		},
		DeleteFunc: func(o interface{}) {
			op.processDelete(ctx, o)
		},
	}, k8scache.Indexers{})

	// Signal cancellation of the main context.
	op.quit = func() {
		cancelFn()
	}

	// Stops running until the context is canceled,
	// which should only happen when op.Shutdown is called.
	informer.Run(ctx.Done())

	return ctx.Err()
}

// Shutdown gracefully shuts down the server.
func (op *Operator) Shutdown() {
	op.Noticef("Shutting down...")

	op.Lock()
	clusters := op.clusters
	op.Unlock()

	// Signal stop of all clusters in each namespace asynchronously.
	for _, clustersInNamespace := range clusters {
		for _, cluster := range clustersInNamespace {
			go cluster.Stop()
		}
	}

	// Block until all controllers have stopped.
	op.wg.Wait()

	// Cancel main context to signal exit.
	op.quit()

	op.Noticef("Bye...")
}
