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

package model

import (
	"testing"
)

const (
	testService1 = "test-service-1.default.svc.cluster.local"
	testService2 = "test-service-2.default.svc.cluster.local"
	testSubset1  = "test-subset-1"
	testSubset2  = "test"
)

func TestMeshNewMesh(t *testing.T) {
	tm := NewMesh()
	if tm == nil {
		t.Error("expecting valid Mesh, found nil")
	}
}

func TestMeshEndpoints(t *testing.T) {
	tm := NewMesh()
	expected := []*Endpoint{}
	t.Run("EmptyMesh", func(t *testing.T) {
		actual := tm.MeshEndpoints(subsetNames)
	})
}

func TestMeshSubsetNames(t *testing.T) {

}

func TestMeshReconcile(t *testing.T) {

}

func TestMeshUpdateSubsets(t *testing.T) {

}

func TestMeshNewEndpoint(t *testing.T) {

}

func TestLabels(t *testing.T) {

}

func TestSetLabels(t *testing.T) {

}

func TestLabels(t *testing.T) {

}

func TestGetLabelValue(t *testing.T) {

}

func TestSetLabelValue(t *testing.T) {

}
