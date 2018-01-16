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

// Package model provies ControllerView which is a platform independent
// abstraction used by Controllers to synchronize the list of service
// endpoints used by the pilot with those available in the platform specific
// registry. It implements all the necessary logic that's used for service
// discovery based on routing rules and endpoint subsets.
// Typical Usage:
//
//   import "istio.io/istio/pilot/model"
//
//   type MyPlatformController struct {
//     ControllerView
//	 }
//   ...
//   pc := MyPlatformController{model.NewControllerView()}
//   ...
//   var serviceEndpoints []*model.ServiceInstances
//   serviceEndpoints = buildYourPlatformServiceEndpointList()
//   pc.Reconcile(serviceEndpoints)
//
package model

import (
	"math"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"

	route "istio.io/api/routing/v1alpha2"
	"istio.io/istio/pkg/log"
)

const (

	// pilotInternalLabelPrefix is the prefix for internally used labels that Pilot
	// uses for look ups non-label attributes of the endpoint.
	pilotInternalLabelPrefix = "config.istio.io/pilot.internal."

	// labelServiceName is an internally used label for fetching ServiceEndpoints
	// by ServiceName.
	labelServiceName = pilotInternalLabelPrefix + "ServiceName"

	// labelEndpointAddress is an internally used label for fetching ServiceEndpoints
	// by the Endpoint's network address.
	labelEndpointAddress = pilotInternalLabelPrefix + "EndpointAddress"

	// labelEndpointProtocol is an internally used label for fetching ServiceEndpoints
	// by the Endpoint's network protocol.
	labelEndpointProtocol = pilotInternalLabelPrefix + "EndpointProtocol"

	// labelEndpointNamedPort is an internally used label for fetching ServiceEndpoints
	// by the Endpoint's named port
	labelEndpointNamedPort = pilotInternalLabelPrefix + "EndpointNamedPort"
)

// ControllerView is a platform independent abstraction used by Controllers
// for maintaining a list of service endpoints used by this Pilot.
//
// Controllers under https://github.com/istio/istio/tree/master/pilot/platform
// update the list of service endpoints in this ControllerView with those
// available in a platform specific service registry.
//
// Under the hoods, the ControllerView implements the necessary
// logic required for endpoint discovery based on routing rules
// and endpoint subsets.
// See https://github.com/istio/api/search?q=in%3Afile+"message+Subset"+language%3Aproto
// This logic includes comparing the updated list provided by Controllers with
// what this view holds and accordingly updating internal structures used for
// endpoint discovery.
type ControllerView struct {

	// serviceEndpoints is a map of all the service instances the Controller
	// has provided to this ControllerView. A service instance is uniquely
	// identified by the mesh global service name, the endpoint address and port.
	//
	// The contraints imposed by ControllerView on ServiceEndpoints produced
	// by Controllers are:
	//
	// - The service name is mesh global i.e. if 2 Controllers have instances
	// with the same service name, all those instances are considered to be
	// semantically identical, modulo the API version and other labels that
	// may differ between instances.
	// - The ServiceEndpoints must be network reachable from _THIS_ Pilot.
	// But otherwise the ControllerView makes no distinction between a
	// ServiceInstance exposed by a single workload or a ServiceInstance exposed
	// by a proxy that exposes a VIP. The VIP itself may be backed by multiple
	// workloads behind that proxy and those workloads are not expected to be
	// directly network reachable from _THIS_ Pilot.
	// - For a given service name, the ServiceInstance's endpoint must be unique
	// across the mesh, meaning 2 Controllers must never produce identical network
	// endpoints (having the same Address and port) for the same service. However
	// in esoteric Istio hybrid configuration scenarios it's possible that 2 different
	// services may end up having the same Network endpoint.
	//
	// The key to this map is a tuple of service name, address and port and is
	// expected to be stable across various string representations of Addresses
	// and ports. For example an IPv6 representation of an IPv4 address and the
	// IPv4 address expressed in normal conventions will result in the same key.
	serviceEndpoints map[string]*ServiceEndpoint

	// subsetDefinitions holds the metadata, basically the set of lable name values.
	// The actual list of ServiceEndpoints that match these subset definitions is
	// stored elsewhere. See subsetEndpoints.
	//
	// The key of this map is the same as what's used for the subsetEndpoints. The value
	// holds key attributes and labels that define the subset.
	subsetDefinitions map[string]*route.Subset

	// subsetEndpoints is a look up from key built off primary properties
	// of a subset to the set of Endpoint Keys. Currently the key to the map
	// is the subset name. TODO: there are plans to allow subset names to be
	//  shared across services at which point the key needs to be
	// <service name>|<subset-name>.
	//
	// The value is a keyset of service endpoint keys. endpointKeys in the set
	// are guaranteed to exist in serviceEndpoints and the corresponding ServiceEndpoint
	// is guaranteed to satisfy the labels in the corresponding Subset
	subsetEndpoints subsetEndpoints

	// reverseLabelMap provides reverse lookup from label name > label value > endpointKeys that
	// match the label name and label value. It's indended primarily for quickly
	// setting up the ControllerView in response to subset configuration change.
	// All endpointKeys associated with each value in the associated
	// valueKeySet are guaranteed to exist in endpointKeySet and the corresponding
	// ServiceEndpoint is guaranteed to have a label with the label name.
	reverseLabelMap labelValues

	// reverseEpSubsets provides a reverse lookup from endpoint key to the set of
	// subsets this endpoint matched to. All subset keys associated with the endpoint
	// are guaranteed to exist in subsetDefinitions.
	reverseEpSubsets endpointSubsets

	// Mutex guards guarantees consistency of updates to members shared across
	// threads. See caveats for Reconcile()
	mu sync.RWMutex
}

// ServiceEndpoint represents a network endpoint associated with a service. Each
// ServiceEndpoint should be expected to be mesh global, i.e. even if two
// controllers have the same service, the network endpoints for each of the Service
// Endpoints must be distinct. However this assumption does not hold across service
// names. In esoteric Istio configurations, it should be possible for two different
// services to expose the same network endpoint.
type ServiceEndpoint struct {
	// The name of the service for which this endpoint was created
	ServiceName string
	// The network address and port for this endpoint. The network address and port
	// must be reachable from _THIS_ Pilot. The ServicePort associated with this
	// Network address is mostly serves for additional ServiceEndpoint metadata.
	// Only the protocol and name are used. The accompanying port number of the
	// ServicePort is ignored.
	NetworkEndpoint NetworkEndpoint
	// Label names and values for this ServiceEndpoint. Controllers should never
	// create labels with the prefix 'config.istio.io/pilot.internal'. Labels
	// supplied with internal prefixes will be silently ignored. Controllers
	// may build labels off workload entities. Examples could be application
	// version, the service availability zone, etc...
	// A general rule of thumb is that the semantics around label names should
	// be consistent across the mesh. For example, a label name "version" should
	// mean the same thing irrespective of the platform Controller. However, it
	// is not necessary for Platform Controllers to expose the same set of
	// label names across services or even across platforms.
	Labels Labels
	// The mesh unique  a tuple of service name, address and port and is
	// expected to be stable across various string representations of Addresses
	// and ports. For example an IPv6 representation of an IPv4 address and the
	// IPv4 address expressed in normal conventions will result in the same key.
	endpointKey string
}

// endpointKeySet is a unique set of endpointKeys
type endpointKeySet map[string]bool

// valueKeySet associates a label value with an endpointKeySet. The endpointKeys
// in the associated endpointKeySet are
// guaranteed to exist in serviceEndpoints and the corresponding
// ServiceEndpoint is guaranteed to have one or more labels with this label value.
type valueKeySet map[string]endpointKeySet

// labelValues is a reverse lookup map from label name > label value > endpointKeys that
// match the label name and label value.
// All endpointKeys associated with each value in the associated
// valueKeySet are guaranteed to exist in endpointKeySet and the corresponding
// ServiceEndpoint is guaranteed to have a label with the label name.
type labelValues map[string]valueKeySet

// subsetKeySet is a unique set of subset keys
type subsetKeySet map[string]bool

// endpointSubsets maps an endpoint key to a key set of matching subsets
type endpointSubsets map[string]subsetKeySet

// subsetEndpoints maps an subset key to a key set of matching endpoint
type subsetEndpoints map[string]endpointKeySet

// NewServiceEndpoint builds a ServiceEndpoint and constructs a stable endpointKey
func NewServiceEndpoint(serviceName string, networkEndpoint NetworkEndpoint, labels Labels) *ServiceEndpoint {
	ip := net.ParseIP(networkEndpoint.Address)
	return &ServiceEndpoint{
		endpointKey:     serviceName + "|" + ip.String() + "|" + strconv.Itoa(networkEndpoint.Port),
		ServiceName:     serviceName,
		NetworkEndpoint: networkEndpoint,
		Labels:          labels,
	}
}

// NewControllerView creates a new empty ControllerView for use by Controller implementations
func NewControllerView() *ControllerView {
	return &ControllerView{
		serviceEndpoints:  map[string]*ServiceEndpoint{},
		subsetDefinitions: map[string]*route.Subset{},
		mu:                sync.RWMutex{},
	}
}

// Reconcile is intended to be called by individual platform registry Controllers to
// update the ControllerView with the latest state of endpoints that make up the
// view. There should be only one thread calling Reconcile and the endpoints
// passed to Reconcile must represent the complete set of endpoints retrieved
// for that platform registry.
func (cv *ControllerView) Reconcile(endpoints []*ServiceEndpoint) {
	// Start out with everything that's provided by the controller and only retain what's not
	// currently in cv.
	epsToAdd := make(map[string]*ServiceEndpoint, len(endpoints))
	for _, ep := range endpoints {
		epsToAdd[ep.endpointKey] = ep
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()
	// Start out with everything in cv and only retain what's not in supplied endpoints.
	epsToDelete := make(map[string]*ServiceEndpoint, len(cv.serviceEndpoints))
	for k, ep := range cv.serviceEndpoints {
		epsToDelete[k] = ep
	}
	for k, expectedEp := range epsToAdd {
		existingEp, found := epsToDelete[k]
		if !found {
			continue // expectedEp will be added
		}
		if !reflect.DeepEqual(*expectedEp, *existingEp) {
			continue // expectedEp will be added, existingEp will be deleted
		}
		// endpoint metadata has remained the same, do not add or delete
		delete(epsToAdd, k)
		delete(epsToDelete, k)
	}
	if len(epsToAdd) == 0 && len(epsToDelete) == 0 {
		return
	}
	for k, delEp := range epsToDelete {
		cv.deleteServiceEndpoint(k, delEp)
	}
	newLabelMappings := labelValues{}
	newSubsetMappings := subsetEndpoints{}
	for k, addEp := range epsToAdd {
		newLabelMappings.addServiceEndpoint(k, addEp)
		cv.serviceEndpoints[k] = addEp
		// By default we create a default subset with the same name as the service
		// This provides a set of subsets that do not have any bearings on endpoint
		// labels.
		// TODO: Revisit default subsets
		newSubsetMappings.AddEndpoint(addEp.ServiceName, k)
	}
	// Verify if existing subset definitions apply for the newly added labels
	for ssKey, subset := range cv.subsetDefinitions {
		matchingEpKeys := newLabelMappings.getKeysMatching(subset.Labels)
		newSubsetMappings.AddEndpointKeys(ssKey, matchingEpKeys)
	}
	// Update controller view for new label mappings and new subset mappings
	cv.reverseLabelMap.addLabelValues(newLabelMappings)
	cv.subsetEndpoints.AddSubsetEndpoints(newSubsetMappings)
}

func (cv *ControllerView) UpdateConfiguration(subsets []*route.Subset, event Event) {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	if event == EventDelete || event == EventUpdate {
		for _, subset := range subsets {
			k := subset.Name
			oldEpKeySet := cv.subsetEndpoints[k]
			for epKey := range oldEpKeySet {
				cv.reverseEpSubsets.DeleteSubset(epKey, k)
			}
			delete(cv.subsetEndpoints, k)
		}
	}
	if event == EventAdd || event == EventUpdate {
		for _, subset := range subsets {
			k := subset.Name
			epKeySet := cv.reverseLabelMap.getKeysMatching(subset.Labels)
			cv.subsetEndpoints.AddEndpointKeys(k, epKeySet)
			for endpointKey := range epKeySet {
				cv.reverseEpSubsets.AddSubset(endpointKey, k)
			}
		}
	}
}

// deleteServiceEndpoint removes all internal references to the endpoint, i.e  from
// serviceEndpoints, subsetEndpoints, reverseLabelMap and reverseEpSubsets. This method
// is expected to be called only from inside reconcile(). The caller is expected
// to lock the resource view before calling this method().
func (cv *ControllerView) deleteServiceEndpoint(k string, ep *ServiceEndpoint) {
	// Remove references from reverseLabelMap
	cv.reverseLabelMap.deleteServiceEndpoint(k, ep)

	// Remove references from reverseEpSubsets and subsetEndpoints
	subsets, subsetsFound := cv.reverseEpSubsets[k]
	if subsetsFound {
		for ssKey := range subsets {
			subsetEps, epsFound := cv.subsetEndpoints[ssKey]
			if epsFound {
				delete(subsetEps, k)
			}
		}
		delete(cv.reverseEpSubsets, k)
	}
	// Remove entry from serviceEndpoints
	delete(cv.serviceEndpoints, k)
}

// deleteServiceEndpoint removes all internal references to the endpoint, i.e  from
// serviceEndpoints, subsetEndpoints, reverseLabelMap and reverseEpSubsets. This method
// is expected to be called only from inside reconcile(). The caller is expected
// to lock the resource view before calling this method().
func (lv labelValues) deleteServiceEndpoint(k string, ep *ServiceEndpoint) {
	// Remove references from reverseLabelMap
	lv.deleteLabel(k, labelServiceName, ep.ServiceName)
	lv.deleteLabel(k, labelEndpointAddress,
		getNormalizedIP(ep.NetworkEndpoint.Address))
	if ep.NetworkEndpoint.ServicePort != nil {
		lv.deleteLabel(k, labelEndpointProtocol,
			string(ep.NetworkEndpoint.ServicePort.Protocol))
		portName := ep.NetworkEndpoint.ServicePort.Name
		if portName == "" {
			portName = strconv.Itoa(ep.NetworkEndpoint.ServicePort.Port)
			lv.deleteLabel(k, labelEndpointNamedPort, portName)
		}
	} else {
		portName := strconv.Itoa(ep.NetworkEndpoint.Port)
		lv.deleteLabel(k, labelEndpointNamedPort, portName)
	}
	for label, value := range ep.Labels {
		lv.deleteLabel(k, label, value)
	}
}

// addServiceEndpoint removes all internal references to the endpoint, i.e  from
// serviceEndpoints, subsetEndpoints, reverseLabelMap and reverseEpSubsets. This method
// is expected to be called only from inside reconcile(). The caller is expected
// to lock the resource view before calling this method().
func (lv labelValues) addServiceEndpoint(k string, ep *ServiceEndpoint) {
	lv.addLabel(k, labelServiceName, ep.ServiceName)
	lv.addLabel(k, labelEndpointAddress,
		getNormalizedIP(ep.NetworkEndpoint.Address))
	if ep.NetworkEndpoint.ServicePort != nil {
		lv.addLabel(k, labelEndpointProtocol,
			string(ep.NetworkEndpoint.ServicePort.Protocol))
		portName := ep.NetworkEndpoint.ServicePort.Name
		if portName == "" {
			portName := strconv.Itoa(ep.NetworkEndpoint.ServicePort.Port)
			lv.addLabel(k, labelEndpointNamedPort, portName)
		}
	} else {
		portName := strconv.Itoa(ep.NetworkEndpoint.Port)
		lv.addLabel(k, labelEndpointNamedPort, portName)
	}
	for label, value := range ep.Labels {
		if strings.HasPrefix(label, pilotInternalLabelPrefix) {
			log.Errorf("Endpoint %s found with prohibited label name '%s'. "+
				"The label value '%s' will be ignored for subset creation", k, label, value)
			continue
		}
		lv.addLabel(k, label, value)
	}
}

// getKeysMatching returns a set of endpointKeys that match the values
// of all labels with a reasonably predictable performance. It does
// this by fetching the Endpoint key sets for for each matching label
// and value then uses the shortest key set for checking whether the
// keys are present in the other key sets.
func (lv labelValues) getKeysMatching(labels Labels) endpointKeySet {
	countLabels := len(labels)
	if countLabels == 0 {
		// There must be at least one label else return nothing
		return endpointKeySet{}
	}
	// Note: 0th index has the smallest keySet
	matchingSets := make([]endpointKeySet, countLabels)
	smallestSetLen := math.MaxInt32
	setIdx := 0
	for l, v := range labels {
		valKeySetMap, found := lv[l]
		if !found {
			// Nothing matched at least one label name
			return endpointKeySet{}
		}
		epKeySet, found := valKeySetMap[v]
		if !found {
			// There were no service keys for this label value
			return endpointKeySet{}
		}
		matchingSets[setIdx] = epKeySet
		lenKeySet := len(epKeySet)
		if lenKeySet < smallestSetLen {
			smallestSetLen = lenKeySet
			if setIdx > 0 {
				swappedMatchingSet := matchingSets[0]
				matchingSets[0] = matchingSets[setIdx]
				matchingSets[setIdx] = swappedMatchingSet
			}
		}
		setIdx++
	}
	if countLabels == 1 {
		return matchingSets[0]
	}
	out := newEndpointKeySet(matchingSets[0])
	for k := range out {
		for setIdx := 1; setIdx < countLabels; setIdx++ {
			_, found := matchingSets[setIdx][k]
			if !found {
				delete(out, k)
				break
			}
		}
	}
	return out
}

// addLabel creates the reverse lookup by label name and label value for the key k.
func (lv labelValues) addLabel(k, labelName, labelValue string) {
	valKeySet, labelNameFound := lv[labelName]
	if !labelNameFound {
		valKeySet = make(valueKeySet)
		lv[labelName] = valKeySet
	}
	keySet, labelValueFound := valKeySet[labelValue]
	if !labelValueFound {
		keySet = make(endpointKeySet)
		valKeySet[labelValue] = keySet
	}
	keySet[k] = true
}

// addLabelValues merges other labelValues with lv.
func (lv labelValues) addLabelValues(other labelValues) {
	for labelName, otherValueKeySet := range other {
		valKeySet, labelNameFound := lv[labelName]
		if !labelNameFound {
			valKeySet = make(valueKeySet)
			lv[labelName] = valKeySet
		}
		for labelValue, otherKeySet := range otherValueKeySet {
			keySet, labelValueFound := valKeySet[labelValue]
			if !labelValueFound {
				keySet = make(endpointKeySet)
				valKeySet[labelValue] = keySet
			}
			for k := range otherKeySet {
				keySet[k] = true
			}
		}
	}
}

// deleteLabel removes the key k from the reverse lookup of the label name and value
func (lv labelValues) deleteLabel(k string, labelName, labelValue string) {
	valKeySet, labelNameFound := lv[labelName]
	if !labelNameFound {
		return
	}
	keySet, labelValueFound := valKeySet[labelValue]
	if !labelValueFound {
		return
	}
	delete(keySet, k)
	if len(keySet) > 0 {
		return
	}
	delete(valKeySet, labelValue)
	if len(valKeySet) > 0 {
		return
	}
	delete(lv, labelName)
}

// newEndpointKeySet creates a copy of fromKs that can be modified without altering fromKs
func newEndpointKeySet(fromKs endpointKeySet) endpointKeySet {
	out := make(endpointKeySet, len(fromKs))
	for k, v := range fromKs {
		if v {
			out[k] = v
		}
	}
	return out
}

// AddSubset adds a subset key to the set of subset keys mapped to the endpoint key
func (es endpointSubsets) AddSubset(endpointKey, subsetKey string) {
	ssKeySet, ssKeySetFound := es[endpointKey]
	if !ssKeySetFound {
		ssKeySet = subsetKeySet{}
		es[endpointKey] = ssKeySet
	}
	ssKeySet[subsetKey] = true
}

// DeleteSubset deletes a subset key from the set of subset keys mapped to the endpoint key
func (es endpointSubsets) DeleteSubset(endpointKey, subsetKey string) {
	ssKeySet, ssKeySetFound := es[endpointKey]
	if !ssKeySetFound {
		return
	}
	delete(ssKeySet, subsetKey)
	if len(ssKeySet) == 0 {
		delete(es, endpointKey)
	}
}

// AddEndpoint adds an endpoint key to the set of endpoint keys mapped to the subset key
func (se subsetEndpoints) AddEndpoint(subsetKey, endpointKey string) {
	epKeySet, epKeySetFound := se[subsetKey]
	if !epKeySetFound {
		epKeySet = endpointKeySet{}
		se[subsetKey] = epKeySet
	}
	epKeySet[endpointKey] = true
}

// AddEndpoint adds an endpoint key to the set of endpoint keys mapped to the subset key
func (se subsetEndpoints) AddEndpointKeys(subsetKey string, epKeySet endpointKeySet) {
	epKeySet, epKeySetFound := se[subsetKey]
	if !epKeySetFound {
		epKeySet = endpointKeySet{}
		se[subsetKey] = epKeySet
	}
	for endpointKey := range epKeySet {
		epKeySet[endpointKey] = true
	}
}

// AddEndpoint adds an endpoint key to the set of endpoint keys mapped to the subset key
func (se subsetEndpoints) AddSubsetEndpoints(other subsetEndpoints) {
	for subsetKey, epKetSet := range other {
		se.AddEndpointKeys(subsetKey, epKetSet)
	}
}

// getNormalizedIP returns a normalized IPv6 address. For example
// given an IPv4-mapped=IPv6 address, it returns the normal IPv4
// address of the kind "10.1.0.2".
func getNormalizedIP(inp string) string {
	ip := net.ParseIP(inp)
	return ip.String()
}
