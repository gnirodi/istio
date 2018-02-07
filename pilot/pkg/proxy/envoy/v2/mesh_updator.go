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

// MeshConfigUpdator is used by config components to update Mesh's configuration.
//
// Implementations of MeshConfigUpdator are not threadsafe. Multi-threaded calls
// should be serialized.
type MeshConfigUpdator interface {
	// UpdateRules updates Mesh with changes to DestinationRules affecting Mesh.
	// It updates Mesh for supplied events, adding, updating and deleting destination
	// rules from this mesh depending on the corresponding ruleChange.Type.
	UpdateRules(ruleChanges []RuleChange) error
}
