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
	"time"
)

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

// Sleeper is intended for use by registries that poll their native data sources at regular intervals.
// Expect Sleeper to be passed along with MeshReconciler while contructing the registry. This is typically
// test friendly and allows for mock Sleepers to be passed during unit tests.
type Sleeper interface {

	// Sleep sleeps for a predetermined period specified during contruction of Sleeper's implementation.
	Sleep()

	// IsDone returns true if Sleeper's user needs to break the periodic cycle to Reconcile.
	// Once IsDone() returns true, subsequent IsDone() will return false until Stop() is called, thus allowing Sleeper to be reused.
	IsDone() bool

	// Stop triggers IsDone to return true.
	Stop()
}

// TimedSleeper is a functional implementation of Sleeper that is used by Registries.
type TimedSleeper struct {
	done          chan bool
	sleepDuration time.Duration
}

// Returns a new TimedSleeper that will sleep for the supplied duration when TimedSleeper.Sleep() is invoked.
func NewTimedSleeper(sleepDuration time.Duration) *TimedSleeper {
	return &TimedSleeper{done: make(chan bool, 1), sleepDuration: sleepDuration}
}

// Sleep implements Sleeper.Sleep().
func (ts *TimedSleeper) Sleep() {
	time.Sleep(ts.sleepDuration)
}

// IsDone implements Sleeper.IsDone().
func (ts *TimedSleeper) IsDone() bool {
	select {
	case <-ts.done:
		return true
	default:
		return false
	}
}

// IsDone implements Sleeper.Stop().
func (ts *TimedSleeper) Stop() {
	ts.done <- true
}
