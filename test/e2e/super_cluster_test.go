// +build e2e

// Copyright 2019 The nats-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"context"
	"testing"
	"time"

	natsv1alpha2 "github.com/nats-io/nats-operator/pkg/apis/nats/v1alpha2"
	"github.com/nats-io/nats-operator/pkg/conf"
	"github.com/nats-io/nats-operator/pkg/constants"
	"k8s.io/api/core/v1"
	watchapi "k8s.io/apimachinery/pkg/watch"
)

func TestCreateServerWithGateways(t *testing.T) {
	var (
		size    = 1
		image   = "synadia/nats-server"
		version = "edge-v2.0.0-RC12"
		nc      *natsv1alpha2.NatsCluster
		err     error
	)
	nc, err = f.CreateCluster(f.Namespace, "test-nats-gw-", size, version,
		func(cluster *natsv1alpha2.NatsCluster) {
			cluster.Spec.ServerImage = image

			cluster.Spec.ServerConfig = &natsv1alpha2.ServerConfig{
				Debug: true,
				Trace: true,
			}

			cluster.Spec.Pod = &natsv1alpha2.PodPolicy{
				AdvertiseExternalIP:         true,
				BootConfigContainerImage:    "wallyqs/nats-boot-config",
				BootConfigContainerImageTag: "0.5.2",
			}

			cluster.Spec.GatewayConfig = &natsv1alpha2.GatewayConfig{
				Name: "minikube",
				Port: 32328,
				Gateways: []*natsv1alpha2.RemoteGatewayOpts{
					&natsv1alpha2.RemoteGatewayOpts{
						Name: "self",
						URL:  "nats://127.0.0.1:32328",
					},
				},
			}
			cluster.Spec.PodTemplate = &v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					ServiceAccountName: "nats-server",
				},
			}
		})
	if err != nil {
		t.Fatal(err)
	}

	// Make sure we cleanup the NatsCluster resource after we're done testing.
	defer func() {
		if err = f.DeleteCluster(nc); err != nil {
			t.Error(err)
		}
	}()

	ctx, done := context.WithTimeout(context.Background(), 120*time.Second)
	defer done()
	err = f.WaitUntilSecretCondition(ctx, nc, func(event watchapi.Event) (bool, error) {
		secret := event.Object.(*v1.Secret)
		conf, ok := secret.Data[constants.ConfigFileName]
		if !ok {
			return false, nil
		}
		config, err := natsconf.Unmarshal(conf)
		if err != nil {
			return false, nil
		}
		if !config.Debug || !config.Trace {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
