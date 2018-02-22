package natsoperator

import (
	"context"
	"fmt"
	"net/http"
	"time"

	k8sapiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8swaitutil "k8s.io/apimachinery/pkg/util/wait"
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
				//
				// # e.g. this supports `kubectl get nats`
				//
				ShortNames: []string{"nats"},
			},
		},
	}
)

// RegisterCRD confirms that the operator can dial into the
// Kubernetes API Server and creates the CRDs in case not currently
// present.
func (op *Operator) RegisterCRD(ctx context.Context) error {
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
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		default:
		}

		result, err := op.kcrdc.ApiextensionsV1beta1().CustomResourceDefinitions().Get(
			CRDObjectFullName,
			k8smetav1.GetOptions{},
		)
		if err != nil {
			if se, ok := err.(*k8sapierrors.StatusError); ok {
				if se.Status().Code == http.StatusNotFound {
					// Continue to retry
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
