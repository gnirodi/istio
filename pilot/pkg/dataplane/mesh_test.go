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

package dataplane

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	route "istio.io/api/routing/v1alpha2"
)

const (
	testService1 = "test-service-1"
	testService2 = "test-service-2"
	testLabelApp = "app"
	testLabelVer = "version"
	testLabelEnv = "environment"
	testLabelExp = "experiment"
	testLabelShd = "shard"
	testSubset1  = "test-subset-1"
	testSubset2  = "test-subset-2"
)

const (
	_                         = iota
	noAttrChange attrToChange = 1 << iota
	diffName
	diffService
	diffNamespace
	diffDomains
	diffAddr
	diffPort
	diffProtocol
	diffLabels
	diffUser
	diffServiceOrder
	diffDomainOrder
)

var (
	allAttrToChange = []attrToChange{
		noAttrChange,
		diffName,
		diffService,
		diffNamespace,
		diffDomains,
		diffAddr,
		diffPort,
		diffProtocol,
		diffLabels,
		diffUser,
		diffServiceOrder,
		diffDomainOrder,
	}
	allTestLabels = []string{
		testLabelApp,
		testLabelVer,
		testLabelEnv,
		testLabelExp,
		testLabelShd,
	}

	// Test domain sets to associate with a service.
	testDomainSets = [][]string{
		{"default.domain-1.com", "my-namespace.my-own-domain-1.com", "my-other-ns.domain-1.com"},
		{"domain-2.com", "my-own-domain-2.com"},
		{"domain-3.com", "my-own-domain-3.com", "my-other-domain-3.com"},
	}

	// applies only to services using testDomainSets[0]
	testNamespaces = []string{"default", "my-namespace", "my-other-ns"}

	testPorts = []uint32{80, 443, 8080}

	testProtocols = []string{"http", "https", "grpc", "redis", "mongo"}
)

// Enums used to tweak attributes while
// building an endpoint for testing
type attrToChange int

func TestMeshNewMesh(t *testing.T) {
	tm := NewMesh()
	if tm == nil {
		t.Error("expecting valid Mesh, found nil")
	}
}

func TestMeshXDS(t *testing.T) {
	tm := NewMesh()
	expectedEps := []*Endpoint{}
	t.Run("EmptyMesh", func(t *testing.T) {
		actual := tm.SubsetEndpoints([]string{"test-service-1.default.domain-1.com"})
		assertEqualEndpointLists(t, expectedEps, actual)
	})
	countEps := 10
	svcSpread := []int{5, 5}    // 2 services each with 50% of endpoints
	lblSpread := []int{3, 2, 7} // 3 labels with each with 3,2,7 values
	expectedSubsets, expectedEps := buildEndpoints(t, countEps, svcSpread, lblSpread, map[string]string{})
	t.Run("AfterReconcile", func(t *testing.T) {
		err := tm.Reconcile(expectedEps)
		if err != nil {
			t.Error(err)
			return
		}
		t.Run("SubsetNames", func(t *testing.T) {
			t.Parallel()
			actualSubsets := tm.SubsetNames()
			assertEqualsSubsetNames(t, expectedSubsets, actualSubsets)
		})
		t.Run("SubsetEndpoints", func(t *testing.T) {
			t.Parallel()
			actualEps := tm.SubsetEndpoints(expectedSubsets)
			assertEqualEndpointLists(t, expectedEps, actualEps)
		})
		t.Run("FetchServiceNoLabels", func(t *testing.T) {
			t.Parallel()
			_, expectedEps := buildEndpoints(t, countEps, svcSpread, lblSpread,
				map[string]string{DestinationService.AttrName(): "test-service-1.default.domain-1.com"})
			actualEps := tm.SubsetEndpoints([]string{"test-service-1.default.domain-1.com"})
			assertEqualEndpointLists(t, expectedEps, actualEps)
		})
		t.Run("FetchNonExistentService", func(t *testing.T) {
			t.Parallel()
			actualEps := tm.SubsetEndpoints([]string{"test-non-existent-service.default.domain-1.com"})
			assertEqualEndpointLists(t, []*Endpoint{}, actualEps)
		})
	})
	t.Run("AfterUpdateRules", func(t *testing.T) {
		err := tm.UpdateRules([]RuleChange{{
			Rule: &route.DestinationRule{
				Name: "test-service-1.default.domain-1.com",
				Subsets: []*route.Subset{&route.Subset{
					Name: "app-1-version-1",
					Labels: map[string]string{
						testLabelApp: "app-1",
						testLabelVer: "version-1",
					},
				}},
			},
			Type: ConfigAdd,
		}})
		if err != nil {
			t.Error(err)
			return
		}
		t.Run("LabeledSubset", func(t *testing.T) {
			t.Parallel()
			_, expectedEps := buildEndpoints(t, countEps, svcSpread, lblSpread,
				map[string]string{
					DestinationService.AttrName(): "test-service-1.default.domain-1.com",
					testLabelApp:                  "app-1",
					testLabelVer:                  "version-1"})
			actualEps := tm.SubsetEndpoints([]string{"test-service-1.default.domain-1.com|app-1-version-1"})
			assertEqualEndpointLists(t, expectedEps, actualEps)
		})
		t.Run("SubsetNames", func(t *testing.T) {
			t.Parallel()
			actualSubsets := tm.SubsetNames()
			expectedSubsets = append(expectedSubsets, "test-service-1.default.domain-1.com|app-1-version-1")
			assertEqualsSubsetNames(t, expectedSubsets, actualSubsets)
		})
		t.Run("SubsetEndpoints", func(t *testing.T) {
			t.Parallel()
			actualEps := tm.SubsetEndpoints(expectedSubsets)
			assertEqualEndpointLists(t, expectedEps, actualEps)
		})
		t.Run("FetchServiceNoLabels", func(t *testing.T) {
			t.Parallel()
			_, expectedEps := buildEndpoints(t, countEps, svcSpread, lblSpread,
				map[string]string{DestinationService.AttrName(): "test-service-1.default.domain-1.com"})
			actualEps := tm.SubsetEndpoints([]string{"test-service-1.default.domain-1.com"})
			assertEqualEndpointLists(t, expectedEps, actualEps)
		})
		t.Run("FetchNonExistentService", func(t *testing.T) {
			t.Parallel()
			actualEps := tm.SubsetEndpoints([]string{"test-non-existent-service.default.domain-1.com"})
			assertEqualEndpointLists(t, []*Endpoint{}, actualEps)
		})
	})
}

// TestMeshEndpointDeepEquals
func TestMeshEndpointDeepEquals(t *testing.T) {
	type epEqualsTestCase struct {
		tcName       string // name of the test case
		attrToChange attrToChange
		expectation  bool
	}
	testCases := make([]epEqualsTestCase, len(allAttrToChange))
	// Single attribute change test cases
	for idx, attrToChange := range allAttrToChange {
		testCases[idx] = epEqualsTestCase{
			attrToChange.String(),
			attrToChange,
			attrToChange == noAttrChange || attrToChange == diffDomainOrder || attrToChange == diffServiceOrder,
		}
	}
	// Other types of attribute changes
	otherTestCases := []epEqualsTestCase{{
		"MultipleAttributes",
		diffNamespace | diffDomains,
		false,
	}}
	testCases = append(testCases, otherTestCases...)
	for _, tc := range testCases {
		t.Run(tc.tcName, func(t *testing.T) {
			firstEp, _ := buildEndpoint(t, noAttrChange, true)
			secondEp, _ := buildEndpoint(t, tc.attrToChange, true)
			actual := reflect.DeepEqual(*firstEp, *secondEp)
			if tc.expectation != actual {
				t.Errorf("Expecting %v, found %v.\nFirst: [%v]\nSecond [%v]",
					tc.expectation, actual, *firstEp, *secondEp)
			}
		})
	}
}

func assertEqualsSubsetNames(t *testing.T, expected, actual []string) {
	expectedSubsets := make(map[string]bool, len(expected))
	for _, subset := range expected {
		expectedSubsets[subset] = true
	}
	actualSubsets := make(map[string]bool, len(actual))
	for _, subset := range actual {
		actualSubsets[subset] = true
	}
	for subset := range expectedSubsets {
		_, found := actualSubsets[subset]
		if !found {
			t.Errorf("expecting subset '%s', none found", subset)
		}
		delete(actualSubsets, subset)
	}
	for subset := range actualSubsets {
		t.Errorf("unexpected subset '%s' found", subset)
	}
}

// assertEqualEndpoints is a utility function used for tests that need to assert
// that the expected and actual service endpoint match, ignoring order of endpoints
// in either array.
func assertEqualEndpointLists(t *testing.T, expected, actual []*Endpoint) {
	expectedSet := map[string]*Endpoint{}
	for _, ep := range expected {
		uid, found := ep.getSingleValuedAttrs()[DestinationUID.AttrName()]
		if !found {
			t.Fatalf("expected ep found with no UID is an indication of bad test data: '%v'", ep)
		}
		expectedSet[uid] = ep
	}
	actualSet := map[string]*Endpoint{}
	for _, ep := range actual {
		uid, found := ep.getSingleValuedAttrs()[DestinationUID.AttrName()]
		if !found {
			t.Errorf("actual ep found with no UID '%s'", epDebugInfo(ep))
			continue
		}
		actualSet[uid] = ep
	}
	for uid, expectedEp := range expectedSet {
		actualEp, found := actualSet[uid]
		if !found {
			t.Errorf("expecting endpoint %s, found none", epDebugInfo(expectedEp))
			continue
		}
		assertEqualEndpoints(t, expectedEp, actualEp)
		delete(actualSet, uid)
	}
	for _, ep := range actualSet {
		t.Errorf("unexpected endpoint found: %s", epDebugInfo(ep))
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
	epListDebugInfo += "]"
	return epListDebugInfo
}

// epDebugInfo prints out a limited set of endpoint attributes for the supplied endpoint.
func epDebugInfo(ep *Endpoint) string {
	var epDebugInfo string
	svcName, found := ep.getSingleValuedAttrs()[DestinationName.AttrName()]
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

func (attr attrToChange) String() string {
	switch attr {
	case noAttrChange:
		return "NoAttrChange"
	case diffName:
		return "DiffName"
	case diffService:
		return "DiffService"
	case diffNamespace:
		return "DiffNamespace"
	case diffDomains:
		return "DiffDomains"
	case diffAddr:
		return "DiffAddr"
	case diffPort:
		return "DiffPort"
	case diffProtocol:
		return "DiffProtocol"
	case diffLabels:
		return "DiffLabels"
	case diffUser:
		return "DiffUser"
	case diffServiceOrder:
		return "DiffServiceOrder"
	case diffDomainOrder:
		return "DiffDomainOrder"

	}
	return (string)(attr)
}

// buildEndpoints builds countEp endpoints for the service with a specified labelSpread. If filterBy is specified,
// it will scope the resultset by the label values in filterBy. if labelSpread contains the magic label skipLables
// the corresponding count of labels will not have any labels. The skipping begins after the count of endpoints with
// the largest label count has completed.
func buildEndpoints(
	t *testing.T, countEp int, svcSpread []int, labelSpread []int, filterBy map[string]string) (subsetNames []string, endpoints []*Endpoint) {
	type svcInfo struct {
		serviceName    string
		countToNextSvc int
		currEpCount    int
	}
	type labelInfo struct {
		labelName   string
		countValues int
		currValue   int
	}

	// Build the service names we want to create
	svcSpec := make([]svcInfo, len(svcSpread))
	for idx, _ := range svcSpec {
		svcSpec[idx] = svcInfo{
			serviceName:    "test-service-" + strconv.Itoa(idx+1),
			countToNextSvc: svcSpread[idx],
		}
	}

	// Build the label names we want to create
	labelSpec := make([]labelInfo, len(labelSpread)%len(allTestLabels))
	for idx, _ := range labelSpec {
		labelSpec[idx] = labelInfo{
			labelName:   allTestLabels[idx],
			countValues: labelSpread[idx],
		}
	}
	subsetNames = []string{}
	var firstOctet int
	currSvcIdx := 0
	for i := 0; i < countEp; i++ {
		sSpec := &svcSpec[currSvcIdx]
		domIdx := currSvcIdx % len(testDomainSets)
		domains := testDomainSets[domIdx]
		var namespace string
		countLblNS := 0
		if domIdx == 0 {
			namespace = testNamespaces[i%len(testNamespaces)]
			countLblNS = 1
		}
		switch {
		case currSvcIdx%2 == 0:
			firstOctet = 10
		default:
			firstOctet = 72
		}
		// Build address of the form 10|72.1.1.1 through 10|72.254.254.254
		addr := strconv.Itoa(firstOctet) + "." +
			strconv.Itoa(((i%16387064)/64516)+1) + "." +
			strconv.Itoa(((i%64516)/254)+1) + "." +
			strconv.Itoa((i%254)+1)
		port := testPorts[i%len(testPorts)]
		protocol := testProtocols[i%len(testProtocols)]
		// UID + Name + Protocol + user+ possibly Namespace + FQDNs + domains + labels
		epLabels := make([]EndpointLabel, 4+countLblNS+(len(domains)*2)+len(labelSpec))
		filterCount := 0
		lblIdx := 0
		epLabels[lblIdx] = EndpointLabel{DestinationUID.AttrName(), "ep-uid-" + strconv.Itoa(i)}
		lblIdx++
		epLabels[lblIdx] = EndpointLabel{DestinationName.AttrName(), sSpec.serviceName}
		lblIdx++
		filterCount = testAndIncrFilterCount(
			filterBy, DestinationName.AttrName(), sSpec.serviceName, filterCount)
		epLabels[lblIdx] = EndpointLabel{DestinationUser.AttrName(),
			sSpec.serviceName + "-user-" + strconv.Itoa(currSvcIdx+1)}
		lblIdx++
		epLabels[lblIdx] = EndpointLabel{DestinationProtocol.AttrName(), protocol}
		lblIdx++
		if countLblNS != 0 {
			epLabels[lblIdx] = EndpointLabel{DestinationNamespace.AttrName(), namespace}
			lblIdx++
		}
		svcNames := make(map[string]bool, len(domains))
		for _, domain := range domains {
			svcName := sSpec.serviceName + "." + domain
			epLabels[lblIdx] =
				EndpointLabel{DestinationService.AttrName(), svcName}
			epLabels[lblIdx+1] = EndpointLabel{DestinationDomain.AttrName(), domain}
			lblIdx += 2
			filterCount = testAndIncrFilterCount(filterBy, DestinationDomain.AttrName(), domain, filterCount)
			filterCount = testAndIncrFilterCount(filterBy, DestinationService.AttrName(), svcName, filterCount)
			svcNames[svcName] = true
		}
		for _, lSpec := range labelSpec {
			labelName := lSpec.labelName
			labelValue := labelName + "-" + strconv.Itoa(lSpec.currValue)
			epLabels[lblIdx] = EndpointLabel{labelName, labelValue}
			lblIdx++
			filterCount = testAndIncrFilterCount(filterBy, labelName, labelValue, filterCount)
		}
		// t.Logf("A: %s, P %d, Lbsl: %v\n", addr, port, epLabels)
		ep, err := NewEndpoint(addr, port, SocketProtocolTCP, epLabels)
		if err != nil {
			t.Fatalf("bad test data: %s", err.Error())
		}
		// If filterBy is provided, only add to endpoints if all filter criteria are met.
		if filterCount == len(filterBy) {
			endpoints = append(endpoints, ep)
			for svcName := range svcNames {
				subsetNames = append(subsetNames, svcName)
			}
		}
		sSpec.currEpCount++
		if sSpec.currEpCount >= sSpec.countToNextSvc {
			sSpec.currEpCount = 0
			currSvcIdx = (currSvcIdx + 1) % len(svcSpread)
		}
		for _, lSpec := range labelSpec {
			lSpec.currValue = (lSpec.currValue + 1) % lSpec.countValues
		}
	}
	return subsetNames, endpoints
}

func testAndIncrFilterCount(filteryBy map[string]string, attrName, attrValue string, currFilterCount int) int {
	if len(filteryBy) > 0 {
		filterValue, found := filteryBy[attrName]
		if found && filterValue == attrValue {
			return currFilterCount + 1
		}
	}
	return currFilterCount
}

func buildEndpoint(t *testing.T, delta attrToChange, assertError bool) (*Endpoint, error) {
	// Setup defaults
	namespace := "default"
	service := testService1
	if delta&diffService == diffService {
		service = testService2
	}
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
		service + ".otherdomain.com",
		service + ".domain2.com",
		"other-host.domain1.com",
	}
	protocol := "http"
	labels := map[string]string{
		"app":        service + "-app",
		"ver":        "1.0",
		"experiment": "1A20",
	}
	user := "some-user" + service
	// Perform required deltas
	if delta&diffAddr == diffAddr {
		uidPart += "11"
		addr += "0"
	}
	if delta&diffNamespace == diffNamespace {
		namespace = "my-namespace"
	}
	if delta&diffPort == diffPort {
		uidPart += "22"
		port += 8000
	}
	if delta&diffProtocol == diffProtocol {
		protocol = "grpc"
	}
	if delta&diffDomains == diffDomains {
		aliases = append(aliases, service+".a-very-different-domain.com")
	}
	if delta&diffLabels == diffLabels {
		labels["experiment"] = "8H22"
	}
	if delta&diffUser == diffUser {
		user = user + "-other"
	}

	fqService := service + ".somedomain.com"
	epLabels := []EndpointLabel{}
	for n, v := range labels {
		epLabels = append(epLabels, EndpointLabel{Name: n, Value: v})
	}
	svcName := service
	if delta&diffName == diffName {
		svcName += "-svc"
	}

	epLabels = append(epLabels, EndpointLabel{Name: DestinationName.AttrName(), Value: svcName})
	epLabels = append(epLabels, EndpointLabel{Name: DestinationService.AttrName(), Value: fqService})

	aliasesToUse := make([]string, len(aliases))
	copy(aliasesToUse, aliases)
	if delta&diffServiceOrder == diffServiceOrder {
		sort.Sort(sort.Reverse(sort.StringSlice(aliasesToUse)))
	}
	for _, alias := range aliases {
		epLabels = append(epLabels, EndpointLabel{Name: DestinationService.AttrName(), Value: alias})
	}

	aliasesForDomains := make([]string, len(aliases))
	copy(aliasesForDomains, aliases)
	if delta&diffDomainOrder == diffDomainOrder {
		sort.Sort(sort.Reverse(sort.StringSlice(aliasesForDomains)))
	}
	for _, alias := range aliasesForDomains {
		subDomains := strings.Split(alias, ".")
		if len(subDomains) < 3 {
			t.Fatalf("bad test data: unable to parse domain from alias '%s'", alias)
		}
		aliasDomain := strings.Join(subDomains[1:], ".")
		epLabels = append(epLabels, EndpointLabel{Name: DestinationDomain.AttrName(), Value: aliasDomain})
	}

	epLabels = append(epLabels, EndpointLabel{Name: DestinationProtocol.AttrName(), Value: protocol})
	epLabels = append(epLabels, EndpointLabel{Name: DestinationUser.AttrName(), Value: user})
	epLabels = append(epLabels, EndpointLabel{
		Name: DestinationUID.AttrName(), Value: "some-controller://" + service + "-app" + uidPart + "." + namespace})

	ep, err := NewEndpoint(addr, port, SocketProtocolTCP, epLabels)
	if err != nil && assertError {
		t.Fatalf("bad test data: %s", err.Error())
	}
	return ep, err
}
