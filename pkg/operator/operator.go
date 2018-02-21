// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	// k8sapiv1 "k8s.io/api/core/v1"
	k8sapiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	k8scrdclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfields "k8s.io/apimachinery/pkg/fields"
	k8swaitutil "k8s.io/apimachinery/pkg/util/wait"
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
	podname string
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
		AddFunc: func(natsClusterObject interface{}) {
			op.Tracef("New NATS Cluster: %+v", natsClusterObject)
		},
		UpdateFunc: func(natsClusterObject interface{}, new interface{}) {
			op.Tracef("Updated NATS Cluster: %+v", natsClusterObject)
		},
		DeleteFunc: func(natsClusterObject interface{}) {
			op.Tracef("Deleted NATS Cluster: %+v", natsClusterObject)
		},
	}, k8scache.Indexers{})

	// FIXME: Ideally should be context here but the `informer`
	// takes a channel instead :/
	informerStopCh := make(chan struct{})

	// Release all resources on shutdown.
	op.quit = func() {
		close(informerStopCh)
		cancelFn()
	}

	// FIXME: Use waitgroup since awaiting for various goroutines now.
	go informer.Run(informerStopCh)

	// Wait until context is done.
	for {
		select {
		case <-ctx.Done():
			op.Noticef("Bye!")
			return ctx.Err()
		}
	}
}

// Shutdown gracefully shuts down the server.
func (op *Operator) Shutdown() {
	op.Noticef("Shutting down...")

	// TODO: Add wait for goroutines to wrap up

	// Cancel main context to signal exit
	op.quit()
}

// RegisterCRD confirms that the operator can dial into the
// Kubernetes API Server and creates the CRDs in case not currently
// present.
func (op *Operator) RegisterCRD(context.Context) error {
	// Ping the server and bail already if there is no connectivity.
	v, err := op.kc.Discovery().ServerVersion()
	if err != nil {
		return err
	}
	op.Noticef("Running on Kubernetes Cluster %v", v)

	// Create the CRD if it does not exists.
	crdc := op.kcrdc.ApiextensionsV1beta1().CustomResourceDefinitions()
	if _, err := crdc.Create(DefaultNATSClusterCRD); err != nil && !k8sapierrors.IsAlreadyExists(err) {
		return err
	}
	op.Noticef("CRD created successfully")

	// Wait for CRD to be ready.
	err = k8swaitutil.Poll(3*time.Second, 10*time.Minute, func() (bool, error) {
		result, err := op.kcrdc.ApiextensionsV1beta1().CustomResourceDefinitions().Get(
			CRDObjectFullName,
			k8smetav1.GetOptions{},
		)
		if err != nil {
			if se, ok := err.(*k8sapierrors.StatusError); ok {
				if se.Status().Code == http.StatusNotFound {
					return false, nil
				}
			}
			return false, err
		}

		// Confirm that the CRD condition has converged
		// to the 'established' state.
		for _, cond := range result.Status.Conditions {
			switch cond.Type {
			case k8sapiextensionsv1beta1.Established:
				if cond.Status == k8sapiextensionsv1beta1.ConditionTrue {
					return true, nil
				}
			case k8sapiextensionsv1beta1.NamesAccepted:
				if cond.Status == k8sapiextensionsv1beta1.ConditionFalse {
					return false, fmt.Errorf("Name conflict: %v", cond.Reason)
				}
			}
		}

		return true, nil
	})
	if err != nil {
		op.Errorf("Gave up waiting for CRD to be ready: %s", err)
	} else {
		op.Noticef("CRD is ready")
	}

	return nil
}
