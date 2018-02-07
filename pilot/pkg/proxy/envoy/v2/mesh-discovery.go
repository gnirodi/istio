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

// MeshDiscovery allows Pilot to implement aggregation of endpoints
// from multiple registries.
//
// Implementations of MeshDiscovery are required to be threadsafe.
type MeshDiscovery interface {
	// SubsetEndpoints implements functionality required for EDS and returns a list of endpoints that match one or more subsets.
	SubsetEndpoints(subsetNames []string) []*Endpoint

	// SubsetNames implements functionality required for CDS and returns a list of all subset names currently configured for this Mesh
	SubsetNames() []string
}
