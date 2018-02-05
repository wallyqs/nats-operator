// natsoperator is a Kubernetes Operator for NATS clusters.
package natsoperator

import (
	"context"
	"net/http"
	"time"

	k8sextensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	k8scrdclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8swaitutil "k8s.io/apimachinery/pkg/util/wait"
	k8sclient "k8s.io/client-go/kubernetes"
	k8srestapi "k8s.io/client-go/rest"
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
	op.quit = func() {
		cancelFn()
	}

	// Setup connection between the Operator and Kubernetes
	// and register CRD to make available API group.
	if err := op.RegisterCRDs(ctx); err != nil {
		return err
	}

	for {
		// TODO: Implement events polling
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
	crd := &k8sextensionsobj.CustomResourceDefinition{
		// ---
		// apiVersion: apiextensions.k8s.io/v1beta1
		// kind: CustomResourceDefinition
		// ...
		TypeMeta: k8smetav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1beta1",
		},
		// metadata:
		//   # name must match the spec fields below, and be
		//   # in the form: <plural>.<group>
		//   name: natsserverclusters.alpha.nats.io
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: "natsserverclusters.alpha.nats.io",
		},
		Spec: k8sextensionsobj.CustomResourceDefinitionSpec{
			// # group name to use for REST API: /apis/<group>/<version>
			// group: alpha.nats.io
			//
			// # version name to use for REST API: /apis/<group>/<version>
			// version: v1alpha2
			//
			// # either Namespaced or Cluster
			// scope: Namespaced
			Group:   "alpha.nats.io",
			Version: "v1alpha2",
			Scope:   k8sextensionsobj.ResourceScope("Namespaced"),
			Names: k8sextensionsobj.CustomResourceDefinitionNames{
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

	crdc := op.kcrdc.ApiextensionsV1beta1().CustomResourceDefinitions()
	if _, err := crdc.Create(crd); err != nil && !k8sapierrors.IsAlreadyExists(err) {
		return err
	}
	op.Noticef("CRD created successfully")

	// Wait for CRD to be ready.
	err = k8swaitutil.Poll(3*time.Second, 10*time.Minute, func() (bool, error) {
		_, err := op.kcrdc.ApiextensionsV1beta1().CustomResourceDefinitions().Get("natsserverclusters.alpha.nats.io", k8smetav1.GetOptions{})
		if err != nil {
			if se, ok := err.(*k8sapierrors.StatusError); ok {
				if se.Status().Code == http.StatusNotFound {
					return false, nil
				}
			}
			return false, err
		}

		return true, nil
	})
	if err != nil {
		op.Errorf("Gave up waiting for CRD to be ready: %s", err)
	}
	op.Noticef("Detected CRD is ready to use")

	return nil
}
