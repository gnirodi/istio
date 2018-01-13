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

// ControllerView is a platform independent abstraction used by Controllers
// to synchronize the list of service endpoints used by the pilot with those
// avaiable in the platform specific registry. It implements all the necessary
// logic that's used for service discovery based on routing rules and endpoint
// subsets. 
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
	"sync"
	
	route "istio.io/api/routing/v1alpha2"
)

const (
    // Label name used internally by ControllerView to fetch ServiceEndpoints
    // by ServiceName. TODO It's possible that the prefix 'config.istio.io/pilot.'
    // is fragile. We should likely introduce some governance to what label names
    // used for Istio can be, to protect customers from stumbling
    // into labeling their Endpoints with names used internally within Istio.	
    labelServiceName 		= "config.istio.io/pilot.ServiceName"

    // Label name used internally by ControllerView to fetch ServiceEndpoints
    // by the Endpoint's Address	
    labelEndpointAddress 	= "config.istio.io/pilot.EndpointAddress"

    // Label name used internally by ControllerView to fetch ServiceEndpoints
    // by the Endpoint's Protocol	
    labelEndpointProtocol 	= "config.istio.io/pilot.EndpointProtocol"

    // Label name used internally by ControllerView to fetch ServiceEndpoints
    // by the Endpoint's Address and named port	
    labelEndpointNamedPort 	= "config.istio.io/pilot.EndpointNamedPort"
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

    // subsetEndpoints is a reverse look up from key built off primary properties
    // of a subset. Currently the key is the subset name. TODO: there are plans to
    // allow subset names to be shared across services at which point the key
    // needs to be <service name>|<subset-name>.
    //
    // The value is a keyset of service endpoint keys. EndpointKeys in the set
    // are guaranteed to exist in serviceEndpoints and the corresponding ServiceEndpoint
    // is guaranteed to satisfy the labels in the corresponding Subset
	subsetEndpoints map[string]endpointKeySet
	
    // reverseLabelMap provides reverse lookup from label name > label value > EndpointKeys that
    // match the label name and label value. It's indended primarily for quickly
    // setting up the ControllerView in response to subset configuration changes without
    // holding write locks for too long. 
    // All EndpointKeys associated with each value in the associated
    // valueKeySet are guaranteed to exist in endpointKeySet and the corresponding
    // ServiceEndpoint is guaranteed to have a label with the label name.
	reverseLabelMap	labelValues
	
	// Mutex guards guarantees consistency of updates to members shared across
	// threads. See caveats for Reconcile()
	mu sync.RWMutex
}

type ServiceEndpoint struct {
	// The mesh unique  a tuple of service name, address and port and is
	// expected to be stable across various string representations of Addresses
	// and ports. For example an IPv6 representation of an IPv4 address and the
	// IPv4 address expressed in normal conventions will result in the same key.    
    EndpointKey		string
    // The name of the service for which this endpoint was created 
    ServiceName		string
    // The network address and port for this endpoint.
    NetworkEndpoint NetworkEndpoint
    Labels			Labels
}

// endpointKeySet is a unique set of EndpointKeys
type endpointKeySet map[string]bool

// valueKeySet associates a label value with an endpointKeySet. The EndpointKeys
// in the associated endpointKeySet are
// guaranteed to exist in serviceEndpoints and the corresponding
// ServiceEndpoint is guaranteed to have one or more labels with this label value.
type valueKeySet map[string]endpointKeySet

// labelValues is a reverse lookup map from label name > label value > EndpointKeys that
// match the label name and label value.
// All EndpointKeys associated with each value in the associated
// valueKeySet are guaranteed to exist in endpointKeySet and the corresponding
// ServiceEndpoint is guaranteed to have a label with the label name.
type labelValues map[string]valueKeySet

func NewServiceEndpoint(serviceName string, networkEndpoint NetworkEndpoint, labels Labels) *ServiceEndpoint {
    ip 			:= net.ParseIP(networkEndpoint.Address)
    return &ServiceEndpoint {
        EndpointKey: 		serviceName + "|" + ip.String() + "|" + strconv.Itoa(networkEndpoint.Port),
        ServiceName: 		serviceName,
        NetworkEndpoint:	networkEndpoint,
        Labels:				labels,
    }
}

// NewControllerView creates a new empty ControllerView for use by Controller implementations
func NewControllerView() *ControllerView {
	return &ControllerView{
		serviceEndpoints:  	map[string]*ServiceEndpoint{},
		subsetDefinitions:	map[string]*route.Subset{},
		mu:               	sync.RWMutex{},
	}
}

// Reconcile is intended to be called by individual platform registry Controllers to
// update the ControllerView with the latest state of endpoints that make up the
// view. There should be only one thread calling Reconcile and the endpoints
// passed to Reconcile must represent the complete set of endpoints retrieved
// for that platform registry.
func (cv *ControllerView) Reconcile(endpoints []*ServiceEndpoint) {
	// No need to synchronize reading cv.serviceEndpoints given this is the only writer
	// thread. All modifications to serviceEndpoints ought to be done after locking cv.mu
	// Start out with everything and only retain what's not in endpoints.
	epsToDelete := make(map[string]*ServiceEndpoint, len(cv.serviceEndpoints))
	for k, ep := range cv.serviceEndpoints {
		epsToDelete[k] = ep
	}    

	epsToAdd := make(map[string]*ServiceEndpoint, len(endpoints))
	// Start out with everything that's provided by the controller and only retain what's not
	// currently in cv.
	for _, ep := range endpoints {
		epsToAdd[ep.EndpointKey] = ep
	}    
    // Only contains what's in cv as well as endpoints but with
    // differing labels.
	epsToUpdate := map[string]*ServiceEndpoint{}
	for k, expectedEp := range epsToAdd {
		existingEp, found := epsToDelete[k]
		if !found {
			continue  // Needs to be added to ControllerView
		}
		if !reflect.DeepEqual(*expectedEp, *existingEp) {
			epsToUpdate[k] = expectedEp
		}
		delete(epsToAdd, k)
		delete(epsToDelete, k)
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	for k, delEp := range epsToDelete {
		cv.reconcileServiceEndpoint(k, delEp, EventDelete)
	}
	for k, addEp := range epsToAdd {
		cv.reconcileServiceEndpoint(k, addEp, EventAdd)
	}
	for k, updEp := range epsToUpdate {
		cv.reconcileServiceEndpoint(k, updEp, EventUpdate)
	}    
}

// reconcileServiceEndpoint is expected to be called only from inside reconcile(). The caller is expected
// to lock the resource view before calling this method()
func (cv *ControllerView) reconcileServiceEndpoint(k string, ep *ServiceEndpoint, event Event) {
    var epLabelAdd, epLabelDelete *ServiceEndpoint
    switch event {
        case EventDelete:
            epLabelDelete = ep
            break
        case EventUpdate:
            epLabelDelete = cv.serviceEndpoints[k]
            epLabelAdd = ep
            break
        case EventAdd:
            epLabelAdd = ep
            break
    }
	if epLabelDelete != nil {
		cv.reverseLabelMap.deleteLabel(k, labelServiceName, epLabelDelete.ServiceName)
		cv.reverseLabelMap.deleteLabel(k, labelEndpointAddress,
		    getNormalizedIP(epLabelDelete.NetworkEndpoint.Address))
		if epLabelDelete.NetworkEndpoint.ServicePort != nil {
			cv.reverseLabelMap.deleteLabel(k, labelEndpointProtocol,
    			string(epLabelDelete.NetworkEndpoint.ServicePort.Protocol))
			portName := epLabelDelete.NetworkEndpoint.ServicePort.Name
			if portName == "" {
			    portName = strconv.Itoa(epLabelDelete.NetworkEndpoint.ServicePort.Port)
    			cv.reverseLabelMap.deleteLabel(k, labelEndpointNamedPort, portName)
			}
		} else {
		    portName := strconv.Itoa(epLabelDelete.NetworkEndpoint.Port)
			cv.reverseLabelMap.deleteLabel(k, labelEndpointNamedPort, portName)
		}
		for label, value := range epLabelDelete.Labels {
			cv.reverseLabelMap.deleteLabel(k, label, value)
		}
		delete(cv.serviceEndpoints, k)
	}
	if epLabelAdd != nil {
		cv.reverseLabelMap.addLabel(k, labelServiceName, epLabelAdd.ServiceName)
		cv.reverseLabelMap.addLabel(k, labelEndpointAddress,
		    getNormalizedIP(epLabelAdd.NetworkEndpoint.Address))
		if epLabelAdd.NetworkEndpoint.ServicePort != nil {
			cv.reverseLabelMap.addLabel(k, labelEndpointProtocol,
    			string(epLabelAdd.NetworkEndpoint.ServicePort.Protocol))
			portName := epLabelAdd.NetworkEndpoint.ServicePort.Name
			if portName == "" {
			    portName := strconv.Itoa(epLabelAdd.NetworkEndpoint.ServicePort.Port)
    			cv.reverseLabelMap.addLabel(k, labelEndpointNamedPort, portName)
			}
		} else {
		    portName := strconv.Itoa(epLabelAdd.NetworkEndpoint.Port)
			cv.reverseLabelMap.addLabel(k, labelEndpointNamedPort, portName)
		}
		for label, value := range epLabelAdd.Labels {
			cv.reverseLabelMap.addLabel(k, label, value)
		}
		cv.serviceEndpoints[k] = epLabelAdd
	}
}

// getKeysMatching returns a set of EndpointKeys that match the values
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

// getNormalizedIP returns a normalized IPv6 address. For example
// given an IPv4-mapped=IPv6 address, it returns the normal IPv4
// address of the kind "10.1.0.2".
func getNormalizedIP(inp string) string {
    ip := net.ParseIP(inp)
    return ip.String()
}
