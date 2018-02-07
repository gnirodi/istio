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

// MeshReconciler is used by service registry adapters to update Mesh endpoints.
// Pilot code usually passes the MeshReconciler at the time of adapter creation.
// Registries that do not support incremental updates, are expected to call
// Reconcile() periodically. Registries that support event based updates should
// call ReconcileDeltas().
//
// Implementations of MeshReconciler are not thread-safe. Multi-threaded calls
// to Reconcile methods should be serialized.
type MeshReconciler interface {
	// Reconcile is intended to be called by individual service registries to
	// update Mesh with the latest list of endpoints that make up the view
	// into it's associated service registry. There should be only one thread
	// calling Reconcile and the endpoints passed to Reconcile must represent
	// the complete set of endpoints retrieved for that environment's service
	// registry. The supplied endpoints should only have been created via
	// NewEndpoint()
	Reconcile(endpoints []*Endpoint) error

	// ReconcileDeltas allows registies to update Meshes incrementally with
	// only those Endpoints that have changed. There should be only one thread
	// calling ReconcileDelta and the endpoints passed to Reconcile must
	// be created via NewEndpoint().
	ReconcileDeltas(endpointChanges []EndpointChange) error
}
