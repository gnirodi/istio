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
)

type PilotRegistryLocality int

const (
	PilotRegistryLocal PilotRegistryLocality = iota
	PilotRegistryRemote
)

// AggregatedMesh aggregates endpoints from local and remote pilots.
type AggregatedMesh struct {
	// Mutex guards guarantees consistency of updates to members shared across
	// threads.
	mu       sync.RWMutex
	regMap   map[string]*PilotRegistry
	regCache *[]*PilotRegistry
}

type PilotRegistry interface {
	ID() string
	PilotLocality() PilotRegistryLocality
	MeshDiscovery
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

func NewLocalPilotRegistry(clusterName string) *LocalPilotRegistry {
	return &LocalPilotRegistry{
		id:   PilotRegistryLocal.String() + "|" + clusterName,
		Mesh: NewMesh(),
	}
}

func NewRemotePilotRegistry(pilotAddress string) *RemotePilotRegistry {
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

// SubsetEndpoints implements MeshDiscovery for the aggregated mesh
func (am *AggregatedMesh) SubsetEndpoints(subsetNames []string) []*Endpoint {
	var registries []*PilotRegistry = *am.registries()
	var out []*Endpoint
	for _, reg := range registries {
		out = append(out, (*reg).SubsetEndpoints(subsetNames)...)
	}
	return out
}

// SubsetNames implements MeshDiscovery
func (am *AggregatedMesh) SubsetNames() []string {
	var registries []*PilotRegistry = *am.registries()
	subsets := map[string]bool{}
	for _, reg := range registries {
		for _, name := range (*reg).SubsetNames() {
			subsets[name] = true
		}
	}
	out := make([]string, 0, len(subsets))
	for subset := range subsets {
		out = append(out, subset)
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
