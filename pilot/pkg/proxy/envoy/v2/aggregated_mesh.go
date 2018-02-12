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
	"sync"

	xdsapi "github.com/envoyproxy/go-control-plane/api"
	"github.com/gogo/protobuf/types"
	"istio.io/istio/pilot/pkg/config/clusterregistry"
)

// PilotRegistryLocality is an enumerated type of registry locality with respect to Pilot.
// The term locality in the context of Pilot registries indicates whether Pilot watches on
// local endpoint events or whether Pilot depends on a remote Pilot for sending it endpoints.
type PilotRegistryLocality int

const (
	// PilotRegistryLocal represents registries that belongs to Pilot and are locally watched for endpoint events.
	PilotRegistryLocal PilotRegistryLocality = iota
	// PilotRegistryRemote represent remote registries and Pilot depends on those RemotePilots to watch for endpoints local to their environment.
	PilotRegistryRemote
)

// AggregatedMesh aggregates endpoints from local and remote pilots.
type AggregatedMesh struct {
	// Mutex guards guarantees consistency of updates to members shared across
	// threads.
	mu sync.RWMutex
	// regMap is a collection of remote and local pilot registries mapped to their respective IDs
	regMap map[string]*PilotRegistry
	// regCache is the cached unmodifiable collection of regMap entries intended for multi-threaded access to the list of registries.
	// The pointer itself is protected and should only be accessed via getRegistries(). Once the pointer is obtained, it
	// can be safely passed around to other methods that need the list and those methods do not have to synchronize on
	// the list.
	regCache *[]*PilotRegistry
	// bootstrapArgs is the initial PilotArgs used to create AggregatedMesh.
	bootstrapArgs *clusterregistry.ClusterStore
}

type PilotRegistry interface {
	ID() string
	PilotLocality() PilotRegistryLocality

	// SubsetEndpoints implements functionality required for EDS and returns a list of endpoints
	// that match one or more subsets.
	SubsetEndpoints(subsetNames []string) []*Endpoint

	// SubsetNames implements functionality required for CDS and returns a list of all subset names currently configured for this Mesh
	SubsetNames() []string

	MeshConfigUpdator
}

type LocalPilotRegistry struct {
	*Mesh
	id string
}

type RemotePilotRegistry struct {
	*Mesh
	id string
}

func NewAggregatedMesh(bootstrapArgs *clusterregistry.ClusterStore) *AggregatedMesh {
	return &AggregatedMesh{
		mu:            sync.RWMutex{},
		regMap:        map[string]*PilotRegistry{},
		regCache:      &[]*PilotRegistry{},
		bootstrapArgs: bootstrapArgs,
	}
}

func NewLocalPilotRegistry(clusterName string) *LocalPilotRegistry {
	return &LocalPilotRegistry{
		id:   PilotRegistryLocal.String() + "|" + clusterName,
		Mesh: NewMesh(),
	}
}

func newRemotePilotRegistry(pilotAddress string) *RemotePilotRegistry {
	return &RemotePilotRegistry{
		id:   PilotRegistryRemote.String() + "|" + pilotAddress,
		Mesh: NewMesh(),
	}
}

func (am *AggregatedMesh) registries() *[]*PilotRegistry {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.regCache
}

func (am *AggregatedMesh) AddLocalRegistry(lmr LocalPilotRegistry) {
	am.mu.Lock()
	defer am.mu.Unlock()
	var mr PilotRegistry = lmr
	am.regMap[lmr.id] = &mr
	cache := make([]*PilotRegistry, 0, len(am.regMap))
	for _, reg := range *am.registries() {
		cache = append(cache, reg)
	}
	am.regCache = &cache
}

// Endpoints implements MeshDiscovery.Endpoints().
// In Envoy's terminology a subset is service cluster.
func (am *AggregatedMesh) Endpoints(serviceClusters []string) *xdsapi.DiscoveryResponse {
	out := &xdsapi.DiscoveryResponse{}
	out.Resources = make([]*types.Any, 0, len(serviceClusters))
	var wg sync.WaitGroup
	registries := *am.registries()
	for idx, serviceCluster := range serviceClusters {
		wg.Add(1)
		go func(serviceCluster string, ptr **types.Any) {
			regEndpoints := make([][]*Endpoint, 0, len(registries))
			totalEndpoints := 0
			for regIdx, reg := range registries {
				regEndpoints[regIdx] = (*reg).SubsetEndpoints([]string{serviceCluster})
				totalEndpoints += len(regEndpoints[regIdx])
			}
			lbEndpoints := make([]*xdsapi.LbEndpoint, 0, totalEndpoints)
			idxLbEps := 0
			for _, epArray := range regEndpoints {
				for _, ep := range epArray {
					lbEndpoints[idxLbEps] = (*xdsapi.LbEndpoint)(ep)
					idxLbEps++
				}
			}
			resource := xdsapi.LocalityLbEndpoints{
				LbEndpoints: lbEndpoints,
			}
			if totalEndpoints > 0 {
				// singleValuedAttr := (*Endpoint)(lbEndpoints[0]).getSingleValuedAttrs()
				resource.Locality = &xdsapi.Locality{
				// TODO
				// Region: lbEndpoints[0].singleValuedAttr[someLabel]
				}
			}
			*ptr, _ = types.MarshalAny(&resource)
		}(serviceCluster, &out.Resources[idx])
	}
	wg.Wait()
	return out
}

// Clusters implements MeshDiscovery.Clusters().
func (am *AggregatedMesh) Clusters() *xdsapi.DiscoveryResponse {
	subsets := map[string]bool{}
	registries := *am.registries()
	for _, reg := range registries {
		for _, name := range (*reg).SubsetNames() {
			subsets[name] = true
		}
	}
	out := &xdsapi.DiscoveryResponse{}
	out.Resources = make([]*types.Any, 0, len(subsets))
	for subset := range subsets {
		resource, _ := types.MarshalAny(&xdsapi.Cluster{Name: subset})
		out.Resources = append(out.Resources, resource)
	}
	return out
}

func (lmr LocalPilotRegistry) PilotLocality() PilotRegistryLocality {
	return PilotRegistryLocal
}

func (rmr RemotePilotRegistry) PilotLocality() PilotRegistryLocality {
	return PilotRegistryRemote
}

func (lmr LocalPilotRegistry) ID() string {
	return lmr.id
}

func (rmr RemotePilotRegistry) ID() string {
	return rmr.id
}

func (l PilotRegistryLocality) String() string {
	switch l {
	case PilotRegistryRemote:
		return "RemoteRegistry"
	default:
		return "LocalRegistry"
	}
}
