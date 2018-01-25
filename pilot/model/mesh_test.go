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
	"reflect"
	"strconv"
	"testing"
)

const (
	testService1 = "test-service-1"
	testService2 = "test-service-2"
	testSubset1  = "test-subset-1"
	testSubset2  = "test-subset-2"
)

// Enums used to tweak attributes while
// building an endpoint for testing
type AttrToChange int

const (
	NOCHANGE       AttrToChange = iota
	DIFF_NAMESPACE AttrToChange = 1 << iota
	DIFF_ALIASES
	DIFF_ADDR
	DIFF_PORT
	DIFF_PROTOCOL
	DIFF_LABELS
	DIFF_USER
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
		actual := tm.MeshEndpoints([]string{testService1})
		assertEqualEndpointLists(t, expected, actual)
	})
}

func TestMeshSubsetNames(t *testing.T) {

}

func TestMeshReconcile(t *testing.T) {

}

func TestMeshUpdateSubsets(t *testing.T) {

}

func TestMeshEndpointDeepEquals(t *testing.T) {
	type epEqualsTestCase struct {
		tcName      string
		epSvc1      string
		deltaSvc1   AttrToChange
		epSvc2      string
		deltaSvc2   AttrToChange
		expectation bool
	}
	testCases := []epEqualsTestCase{
		epEqualsTestCase{
			"SameEndpoint",
			testService1,
			NOCHANGE,
			testService1,
			NOCHANGE,
			true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.tcName, func(t *testing.T) {
			firstEp, _ := buildEndpoint(t, tc.epSvc1, tc.deltaSvc1, true)
			secondEp, _ := buildEndpoint(t, tc.epSvc2, tc.deltaSvc2, true)
			actual := reflect.DeepEqual(*firstEp, *secondEp)
			if tc.expectation != actual {
				t.Errorf("Expecting %v, found %v. First [%v] Second [%v]",
					tc.expectation, actual, *firstEp, *secondEp)
			}
		})
	}
}

func buildEndpoint(t *testing.T, service string, delta AttrToChange, assertError bool) (*Endpoint, error) {
	// Setup defaults
	namespace := "default"
	var uidPart, addr string
	if service == testService1 {
		uidPart = "0A0A"
		addr = "10.4.1.1"
	} else {
		uidPart = "B0B0"
		addr = "10.4.4.1"
	}
	var port uint32 = 80
	aliases := []string{
		service + ".domain1.com",
		service + ".domain2.com",
	}
	protocol := ProtocolHTTP
	labels := Labels{
		"app":        service + "-app",
		"ver":        "1.0",
		"experiment": "1A20",
	}
	user := "some-user" + service
	// Perform required deltas
	if delta&DIFF_ADDR == DIFF_ADDR {
		uidPart += "11"
		addr += "0"
	}
	if delta&DIFF_NAMESPACE == DIFF_NAMESPACE {
		namespace = "my-namespace"
	}
	if delta&DIFF_PORT == DIFF_PORT {
		uidPart += "22"
		port += 8000
	}
	if delta&DIFF_PROTOCOL == DIFF_PROTOCOL {
		protocol = ProtocolGRPC
	}
	if delta&DIFF_ALIASES == DIFF_ALIASES {
		aliases = append(aliases, service+"domain3.com")
	}
	if delta&DIFF_LABELS == DIFF_LABELS {
		labels["experiment"] = "8H22"
	}
	fqService := service + "." + namespace + ".svc.cluster.local"
	ep, err := NewEndpoint("some-controller://"+service+"-app"+uidPart+"."+namespace, fqService, aliases, addr, port, protocol, labels, user)
	if err != nil && assertError {
		t.Fatalf("Bad test data: %s", err.Error())
	}
	return ep, err
}

// assertEqualEndpoints is a utility function used for tests that need to assert
// that the expected and actual service endpoint match, ignoring order of endpoints
// in either array.
func assertEqualEndpointLists(t *testing.T, expected, actual []*Endpoint) {
	expectedSet := map[string]*Endpoint{}
	for _, ep := range expected {
		uid, found := ep.getLabelValue(UID.stringValue())
		if !found {
			t.Fatalf("expected ep found with no UID is an indication of bad test data: '%v'", ep)
		}
		expectedSet[uid] = ep
	}
	actualSet := map[string]*Endpoint{}
	for _, ep := range actual {
		uid, found := ep.getLabelValue(UID.stringValue())
		if !found {
			t.Errorf("actual ep found with no UID '%s'", epDebugInfo(ep))
			continue
		}
		expectedSet[uid] = ep
	}
	hasErrors := false
	for uid, expectedEp := range expectedSet {
		actualEp, found := actualSet[uid]
		if !found {
			hasErrors = true
			t.Errorf("expecting endpoint %s, found none", epDebugInfo(expectedEp))
			continue
		}
		assertEqualEndpoints(t, expectedEp, actualEp)
		delete(actualSet, uid)
	}
	if len(actualSet) > 0 {
		for _, ep := range actualSet {
			hasErrors = true
			t.Errorf("unexpected endpoint found: %s", epDebugInfo(ep))
		}
	}
	if hasErrors {
		t.Errorf("mismatched expectations, expected service instances: %s, actual %s", epListDebugInfo(expected), epListDebugInfo(actual))
	}
}

// epListDebugInfo prints out a limited set of endpoint attributes in the supplied endpoint list.
func epListDebugInfo(epList []*Endpoint) string {
	epListDebugInfo := "["
	for _, ep := range epList {
		if ep == nil {
			continue
		}
		if len(epListDebugInfo) > 1 {
			epListDebugInfo += ", "
		}
		epListDebugInfo += epDebugInfo(ep)
	}
	return epListDebugInfo
}

// epDebugInfo prints out a limited set of endpoint attributes for the supplied endpoint.
func epDebugInfo(ep *Endpoint) string {
	var epDebugInfo string
	svcName, found := ep.getLabelValue(SERVICE.stringValue())
	if found {
		epDebugInfo = svcName
	}
	if ep.Endpoint.GetAddress() != nil && ep.Endpoint.GetAddress().GetSocketAddress() != nil {
		addr := ep.Endpoint.GetAddress().GetSocketAddress()
		if epDebugInfo != "" {
			epDebugInfo += "|"
		}
		epDebugInfo += addr.Address
		if epDebugInfo != "" {
			epDebugInfo += "|"
		}
		if addr.GetPortValue() != 0 {
			epDebugInfo += strconv.Itoa((int)(addr.GetPortValue()))
		}
	}
	return epDebugInfo
}

// assertEqualEndpoint is a utility function used for tests that need to assert
// that the expected and actual service endpoints match.
func assertEqualEndpoints(t *testing.T, expected, actual *Endpoint) {
	if !reflect.DeepEqual(*expected, *actual) {
		t.Errorf("Expected endpoint: %v, Actual %v", expected, actual)
	}
}
