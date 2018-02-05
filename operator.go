// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"
	"fmt"
	"net/http"
	"time"

	k8sapiv1 "k8s.io/api/core/v1"
	k8sapiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	k8scrdclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8swaitutil "k8s.io/apimachinery/pkg/util/wait"
	k8sclient "k8s.io/client-go/kubernetes"
	k8srestapi "k8s.io/client-go/rest"
	// k8swatch "k8s.io/apimachinery/pkg/watch"
	k8sfields "k8s.io/apimachinery/pkg/fields"
	k8scache "k8s.io/client-go/tools/cache"
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

	// Kubernetes Pod Namespace
	ns string

	// Kubernetes Pod Name
	podname string
}

// Run starts the main loop.
func (op *Operator) Run(ctx context.Context) error {
	op.Noticef("Starting NATS Server Kubernetes Operator v%s", Version)

	// Setup configuration for when operator runs within
	// Kubernetes and the API client for making requests, then
	// prepare setting up the CRD in in case it does not exist
	// already.
	cfg, err := k8srestapi.InClusterConfig()
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

	op.Debugf("Kubernetes Cluster Config: %+v", cfg)
	op.Debugf("Kubernetes Client: %+v", kc)
	op.Debugf("Kubernetes Extensions Client: %+v", kcrdc)

	// Set up cancellation context for the main loop.
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	// op.quit = func() {
	// 	cancelFn()
	// }

	// Setup connection between the Operator and Kubernetes
	// and register CRD to make available API group.
	if err := op.RegisterCRDs(ctx); err != nil {
		return err
	}

	// Event Handlers -----------------------------------------------------

	// What is this...
	podListWatcher := k8scache.NewListWatchFromClient(
		kc.CoreV1().RESTClient(), // Have to generate this???
		"natsserverclusters",
		op.ns,
		k8sfields.Everything(),
	)

	indexer, informer := k8scache.NewIndexerInformer(podListWatcher, &k8sapiv1.Pod{}, 0, k8scache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			op.Noticef("New! %+v", pod)
			// key, err := cache.MetaNamespaceKeyFunc(obj)
			// if err == nil {
			// 	queue.Add(key)
			// }
		},
		UpdateFunc: func(pod interface{}, new interface{}) {
			op.Noticef("Updated! %+v", pod)
			// key, err := cache.MetaNamespaceKeyFunc(new)
			// if err == nil {
			// 	queue.Add(key)
			// }
		},
		DeleteFunc: func(pod interface{}) {
			op.Noticef("Bye! %+v", pod)
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			// key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			// if err == nil {
			// 	queue.Add(key)
			// }
		},
	}, k8scache.Indexers{})
	op.Debugf("============== Informer: %+v", informer)
	op.Debugf("============== Indexer: %+v", indexer)

	// source := cache.NewListWatchFromClient(
	// 	c.Config.EtcdCRCli.EtcdV1beta2().RESTClient(),
	// 	"natsserverclusters",
	// 	c.Config.Namespace,
	// 	fields.Everything())

	// _, informer := cache.NewIndexerInformer(source, &api.EtcdCluster{}, 0, cache.ResourceEventHandlerFuncs{
	// 	AddFunc:    c.onAddEtcdClus,
	// 	UpdateFunc: c.onUpdateEtcdClus,
	// 	DeleteFunc: c.onDeleteEtcdClus,
	// }, cache.Indexers{})

	// Ideally should be context but the `informer` takes a channel.
	informerStopCh := make(chan struct{})

	// Release all resources on shutdown.
	op.quit = func() {
		close(informerStopCh)
		cancelFn()
	}

	// TODO: Waitgroup since awaiting for various goroutines now
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

	// Done.
	op.quit()
}

// RegisterCRDs confirms that the operator can dial into the
// Kubernetes API Server and creates the CRDs in case not currently
// present.
func (op *Operator) RegisterCRDs(context.Context) error {
	// Ping the server and bail already if there is no connectivity.
	v, err := op.kc.Discovery().ServerVersion()
	if err != nil {
		return err
	}
	op.Noticef("Running on Kubernetes Cluster %v", v)

	// Create the CRD if it does not exists.
	crdc := op.kcrdc.ApiextensionsV1beta1().CustomResourceDefinitions()
	if _, err := crdc.Create(NATSClusterCRD()); err != nil && !k8sapierrors.IsAlreadyExists(err) {
		return err
	}
	op.Noticef("CRD created successfully")

	// Wait for CRD to be ready.
	err = k8swaitutil.Poll(3*time.Second, 10*time.Minute, func() (bool, error) {
		result, err := op.kcrdc.ApiextensionsV1beta1().CustomResourceDefinitions().Get(
			"natsserverclusters.alpha.nats.io",
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
	}
	op.Noticef("CRD is ready")

	return nil
}

// NATSClusterCRD returns a representation of the CRD to register.
func NATSClusterCRD() *k8sapiextensionsv1beta1.CustomResourceDefinition {
	return &k8sapiextensionsv1beta1.CustomResourceDefinition{
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
		//   name: natsserverclusters.nats.io
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: "natsserverclusters.nats.io",
		},
		Spec: k8sapiextensionsv1beta1.CustomResourceDefinitionSpec{
			// # group name to use for REST API: /apis/<group>/<version>
			// group: alpha.nats.io
			//
			// # version name to use for REST API: /apis/<group>/<version>
			// version: v1alpha2
			//
			// # either Namespaced or Cluster
			// scope: Namespaced
			Group:   "nats.io",
			Version: "v1alpha2",
			Scope:   k8sapiextensionsv1beta1.ResourceScope("Namespaced"),
			Names: k8sapiextensionsv1beta1.CustomResourceDefinitionNames{
				// # plural name to be used in the
				// # URL:
				// # /apis/<group>/<version>/<plural>
				// plural: natsserverclusters
				Plural: "natsserverclusters",

				// # singular name to be used as an
				// # alias on the CLI and for display
				// singular: natsserverclusters
				Singular: "natsservercluster",

				// # kind is normally the CamelCased singular type.
				// kind: NatsServerCluster
				Kind: "NatsServerCluster",

				// # shortNames allow shorter string
				// # to match your resource on the CLI
				// - shortNames: ["nsc"]
				ShortNames: []string{"nsc"},
			},
		},
	}
}
