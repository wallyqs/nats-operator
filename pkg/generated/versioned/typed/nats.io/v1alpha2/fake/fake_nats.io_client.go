// Boilerplate
package fake

import (
	v1alpha2 "github.com/nats-io/nats-kubernetes/operators/nats-server/pkg/generated/versioned/typed/nats.io/v1alpha2"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeNatsV1alpha2 struct {
	*testing.Fake
}

func (c *FakeNatsV1alpha2) NatsClusters(namespace string) v1alpha2.NatsClusterInterface {
	return &FakeNatsClusters{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeNatsV1alpha2) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
