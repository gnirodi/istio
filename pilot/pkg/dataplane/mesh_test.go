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
	"math"
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
	countEps, countSvcs, countSubsetsPerSvc := 32, 2, 2
	rules, subsets, expectedEps := buildTestEndpoints(t, countEps, countSvcs, countSubsetsPerSvc)
	expectedSubsets := []string{}
	for i := 0; i < countSvcs; i++ {
		servicePrefix := "test-service-" + strconv.Itoa(i+1)
		for _, domain := range testDomainSets[i] {
			expectedSubsets = append(expectedSubsets, servicePrefix+"."+domain)
		}
	}
	err := tm.Reconcile(expectedEps)
	if err != nil {
		t.Errorf("unable to reconcile(): %v", err)
		return
	}
	t.Run("AfterReconcile", func(t *testing.T) {
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
			actualEps := tm.SubsetEndpoints([]string{"test-service-1.default.domain-1.com"})
			assertEqualEndpointLists(t, expectedEps[0:countEps/countSvcs], actualEps)
		})
		t.Run("FetchNonExistentService", func(t *testing.T) {
			t.Parallel()
			actualEps := tm.SubsetEndpoints([]string{"test-non-existent-service.default.domain-1.com"})
			assertEqualEndpointLists(t, []*Endpoint{}, actualEps)
		})
	})
	t.Run("AfterUpdateRules", func(t *testing.T) {
		err := tm.UpdateRules([]RuleChange{{
			Rule: rules[0],
			Type: ConfigAdd,
		}})
		if err != nil {
			t.Error(err)
			return
		}
		t.Run("LabeledSubset", func(t *testing.T) {
			t.Parallel()
			actualEps := tm.SubsetEndpoints([]string{
				"test-service-1.default.domain-1.com|" + subsets[0].Name})
			assertEqualEndpointLists(t, expectedEps[0:countEps/(countSvcs*countSubsetsPerSvc)], actualEps)
		})
		t.Run("SubsetNames", func(t *testing.T) {
			t.Parallel()
			for i := 0; i < countSubsetsPerSvc; i++ {
				for _, subset := range rules[0].Subsets {
					expectedSubsets = append(expectedSubsets, rules[0].Name+"|"+subset.Name)
				}
			}
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
			actualEps := tm.SubsetEndpoints([]string{"test-service-1.default.domain-1.com"})
			assertEqualEndpointLists(t, expectedEps[0:countEps/countSvcs], actualEps)
		})
		t.Run("FetchNonExistentService", func(t *testing.T) {
			t.Parallel()
			actualEps := tm.SubsetEndpoints([]string{"test-non-existent-service.default.domain-1.com"})
			assertEqualEndpointLists(t, []*Endpoint{}, actualEps)
		})
	})
	// Uncomment for debugging!
	//	t.Logf(
	//		"tm.reverseAttrMap:\n%v\n\ntm.reverseEpSubsets:\n%v\n\ntm.subsetEndpoints:\n%v\n\ntm.subsetDefinitions:\n%v\n\ntm.allEndpoints:\n%v\n\n",
	//		tm.reverseAttrMap, tm.reverseEpSubsets, tm.subsetEndpoints, tm.subsetDefinitions, tm.allEndpoints)
}

/*
func BenchmarkMeshReconcile(b *testing.B) {
	svcSpread := []int{5, 5}    // 2 services each with 50% of endpoints
	lblSpread := []int{3, 2, 7} // 3 labels with each with 3,2,7 values
	_, testEps, _ := buildEndpoints(b, 10000, svcSpread, lblSpread, map[string]string{})
	tm := NewMesh()
	err := tm.Reconcile(testEps)
	if err != nil {
		b.Error(err)
		return
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = tm.Reconcile(testEps)
		if err != nil {
			b.Error(err)
			return
		}
	}
}

func BenchmarkMeshUpdateRules(b *testing.B) {
	svcSpread := []int{5, 5}    // 2 services each with 50% of endpoints
	lblSpread := []int{3, 2, 7} // 3 labels with each with 3,2,7 values
	_, testEps, _ := buildEndpoints(b, 1000000, svcSpread, lblSpread, map[string]string{})
	tm := NewMesh()
	err := tm.Reconcile(testEps)
	if err != nil {
		b.Error(err)
		return
	}
	ruleChanges := []RuleChange{{
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
	}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = tm.UpdateRules(ruleChanges)
		if err != nil {
			b.Error(err)
			return
		}
		b.StopTimer()
		actualEps := tm.SubsetEndpoints([]string{"test-service-1.default.domain-1.com|app-1-version-1"})
		if len(actualEps) != 66667 {
			b.Errorf("test failed, expected 500K endpoints, found %d", len(actualEps))
			return
		}
		b.StartTimer()
	}
}

func bmMeshSubsetEndpointsHelper(b *testing.B, epsCount int, svcSpread []int) {
	lblSpread := []int{3, 2, 7, 11, 13, 17, 19, 23, 29}
	_, testEps, expectedFilteredEps := buildEndpoints(b, epsCount, svcSpread, lblSpread, map[string]string{
		DestinationService.AttrName(): "test-service-1.default.domain-1.com",
		testLabelApp:                  "app-1",
		testLabelVer:                  "version-1",
	})
	expectedCount := len(expectedFilteredEps)
	tm := NewMesh()
	err := tm.Reconcile(testEps)
	if err != nil {
		b.Error(err)
		return
	}
	ruleChanges := []RuleChange{{
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
	}}
	err = tm.UpdateRules(ruleChanges)
	if err != nil {
		b.Error(err)
		return
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		actualEps := tm.SubsetEndpoints([]string{"test-service-1.default.domain-1.com|app-1-version-1"})
		if len(actualEps) != len(expectedFilteredEps) {
			b.Errorf("test failed, expected %d endpoints, found %d", expectedCount, len(actualEps))
			return
		}
	}
	// The computed benchmark should match the name!!!
	computedBmName := fmt.Sprintf("BenchmarkMeshSubsetEndpoints_EP_%d_%d", epsCount, expectedCount)
	if b.Name() != computedBmName {
		b.Errorf("Actual benchmark '%s' is out of sync with computed benchmark '%s'", b.Name(), computedBmName)
	}
}
func BenchmarkMeshSubsetEndpoints_EP_1000000_1617(b *testing.B) {
	// Benchmark: Resultset 0.0067% of 1 Million endpoints
	// Use multiples of 100 for each service spread value.
	bmMeshSubsetEndpointsHelper(b, 1000000, []int{100, 100, 100, 100, 10000})
}
*/

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
			t.Errorf("expecting endpoint\nShortForm: %s\nLongForm  : %s\nfound none", epDebugInfo(expectedEp), *expectedEp)
			continue
		}
		assertEqualEndpoints(t, expectedEp, actualEp)
		delete(actualSet, uid)
	}
	for _, ep := range actualSet {
		t.Errorf("unexpected endpoint found: %s", epDebugInfo(ep))
	}
	if len(expected) != len(actual) {
		t.Errorf("expected endpoint count: %d do not tally with actual count: %d", len(expected), len(actual))
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

func buildTestEndpoints(t testing.TB, cntEps, cntSvcs, cntSubsetsPerSvc int) ([]*route.DestinationRule,
	[]*route.Subset, []*Endpoint) {
	type labelSpec struct {
		labelName   string
		countValues int
		currValue   int
	}
	maxLblValues := []int{2, 3, 5, 7, 11, 13, 17, 19, 23, 29}
	currLblValues := make([]labelSpec, len(maxLblValues))
	for idx, _ := range currLblValues {
		labelInfo := &currLblValues[idx]
		labelInfo.labelName = "label-" + strconv.Itoa(idx+1)
		labelInfo.countValues = maxLblValues[idx]
	}

	ttlSubsets := cntSvcs * cntSubsetsPerSvc
	outRules := make([]*route.DestinationRule, cntSvcs)
	outSubsets := make([]*route.Subset, ttlSubsets)
	outEndpoints := make([]*Endpoint, cntEps)

	// Assuming fixed endpoints per subsets (only for endpoint counts < 1000)
	epsForSS := cntEps / ttlSubsets
	// For large values of endpoint counts assume normal distribution
	sig := -3.0
	sigIncr := (float64)(ttlSubsets) / (float64)(cntEps) * 6.0
	prevCdf := 0.0

	epIdx := 0
	ssIdx := 0
	for svcIdx := 0; svcIdx < cntSvcs; svcIdx++ {
		serviceName := "test-service-" + strconv.Itoa(svcIdx+1)
		domIdx := svcIdx % len(testDomainSets)
		svcDomains := testDomainSets[domIdx]
		rule := &route.DestinationRule{
			Name:    serviceName + "." + svcDomains[0],
			Subsets: make([]*route.Subset, cntSubsetsPerSvc),
		}
		outRules[svcIdx] = rule
		var namespace string
		countLblNS := 0
		if domIdx == 0 {
			namespace = testNamespaces[svcIdx%len(testNamespaces)]
			countLblNS = 1
		}
		var firstOctet int
		switch {
		case svcIdx%2 == 0:
			firstOctet = 10
		default:
			firstOctet = 72
		}
		for svcSSIdx := 0; svcSSIdx < cntSubsetsPerSvc; svcSSIdx, ssIdx = svcSSIdx+1, ssIdx+1 {
			ssIdx := (svcSSIdx * cntSubsetsPerSvc) + svcSSIdx
			subset := &route.Subset{
				Name:   "subset-" + strconv.Itoa(svcSSIdx),
				Labels: make(map[string]string, len(currLblValues)),
			}
			rule.Subsets[svcSSIdx] = subset
			outSubsets[ssIdx] = subset
			// UID + Protocol + Name + user + possibly Namespace + FQDNs + domains + labels
			epLabels := make([]EndpointLabel, 4+countLblNS+(len(svcDomains)*2)+len(currLblValues))
			lblIdx := 2 // UID, Protocol are endpoint specific
			epLabels[lblIdx] = EndpointLabel{DestinationName.AttrName(), serviceName}
			lblIdx++
			epLabels[lblIdx] = EndpointLabel{DestinationUser.AttrName(),
				serviceName + "-user-" + strconv.Itoa(svcIdx+1)}
			lblIdx++
			if countLblNS != 0 {
				epLabels[lblIdx] = EndpointLabel{DestinationNamespace.AttrName(), namespace}
				lblIdx++
			}
			for _, domain := range svcDomains {
				epLabels[lblIdx] =
					EndpointLabel{DestinationService.AttrName(), serviceName + "." + domain}
				epLabels[lblIdx+1] = EndpointLabel{DestinationDomain.AttrName(), domain}
				lblIdx += 2
			}
			// Fix the labels for this subset
			for liIdx, _ := range currLblValues {
				labelInfo := &currLblValues[liIdx]
				labelName := labelInfo.labelName
				labelInfo.currValue += 1
				labelValue := labelName + "-" + strconv.Itoa(labelInfo.currValue)
				subset.Labels[labelName] = labelValue
				epLabels[lblIdx] = EndpointLabel{labelName, labelValue}
				lblIdx++
			}
			// Override endpoints per subset for large values of cntEps
			if cntEps >= 1000 {
				sig += sigIncr
				currCdf := math.Erfc(-sig/math.Sqrt2) / 2.0
				epsForSS = (int)(math.Ceil((currCdf - prevCdf) * (float64)(cntEps)))
				prevCdf = currCdf
			}
			for epSSIdx := 0; epSSIdx < epsForSS; epIdx, epSSIdx = epIdx+1, epSSIdx+1 {
				// Build address of the form 10|72.1.1.1 through 10|72.254.254.254
				addr := strconv.Itoa(firstOctet) + "." +
					strconv.Itoa(((epIdx%16387064)/64516)+1) + "." +
					strconv.Itoa(((epIdx%64516)/254)+1) + "." +
					strconv.Itoa((epIdx%254)+1)
				port := testPorts[epIdx%len(testPorts)]
				// Add the endpoint specific labels: UID + Protocol
				epLabels[0] = EndpointLabel{DestinationUID.AttrName(),
					"ep-uid-" + strconv.Itoa(epIdx)}
				epLabels[1] = EndpointLabel{DestinationProtocol.AttrName(),
					testProtocols[epIdx%len(testProtocols)]}
				// Uncomment for debugging test data build logic
				// t.Logf("Lbsl: %v\n", epLabels)
				ep, err := NewEndpoint(addr, port, SocketProtocolTCP, epLabels)
				if err != nil {
					t.Fatalf("bad test data: %s", err.Error())
				}
				outEndpoints[epIdx] = ep
			}
		}
	}
	return outRules, outSubsets, outEndpoints
}

func buildEndpoint(t testing.TB, delta attrToChange, assertError bool) (*Endpoint, error) {
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
