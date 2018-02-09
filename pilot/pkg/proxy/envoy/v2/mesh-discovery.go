// Copyright 2018 Istio Authors
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

package v2

import (
	xdsapi "github.com/envoyproxy/go-control-plane/api"
)

// MeshDiscovery is is the interface that adapts Envoy's v2 xDS APIs to Istio's discovery APIs
// For Envoy terminology: https://www.envoyproxy.io/docs/envoy/latest/api-v2/api
// For Istio terminology:
// from multiple registries.
//
// Implementations of MeshDiscovery are required to be threadsafe.
type MeshDiscovery interface {
	// Endpoints implements EDS and returns a list of endpoints by subset for the list of supplied subsets.
	// In Envoy's terminology a subset is service cluster.
	Endpoints(serviceClusters []string) []*xdsapi.LocalityLbEndpoints

	// Clusters implements functionality required for CDS and returns a list of all service clusters names currently configured for this Mesh
	Clusters() []xdsapi.Cluster
}
