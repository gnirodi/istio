// Copyright 2017 Istio Authors
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

package aggregate

import (
	"encoding/hex"
	"net"

	"istio.io/istio/pilot/model"
	//   "istio.io/istio/pilot/platform"
)

const (
	// Labels used for xDS internals use only. These labels are never exposed in
	// external xDS interfaces.
	labelXdsPrefix         = "config.istio.io/xds"
	labelServicePrefix     = labelXdsPrefix + "Service."
	labelServiceName       = labelServicePrefix + "name"
	labelServiceDNS        = labelServicePrefix + "dns"
	labelServiceVIP        = labelServicePrefix + "vip"
	labelInstancePrefix    = labelXdsPrefix + "ServiceInstance."
	labelInstanceService   = labelInstancePrefix + "service"
	labelInstanceIP        = labelInstancePrefix + "ip"
	labelInstancePort      = labelInstancePrefix + "port"
	labelInstanceNamedPort = labelInstancePrefix + "namedPort"
)

// Key used to track the service reference maintained by a registry in this controller
// Format is: [service name][platformRegistry]
type serviceKey string

// A unique set of service keys
type serviceKeySet map[serviceKey]bool

// A map of the value of a label to the keys of service that have that label value
type serviceValueSet map[string]serviceKeySet

// Key used to track the service instance maintained by a registry
// Note there be exactly one endpoint for a service with the combination of
// IP address and port
// Format is: [service name][hex value of IP address][hex value of port number]
type serviceInstanceKey string

// A unique set of service instance keys
type serviceInstanceKeySet map[serviceInstanceKey]bool

// A map of the value of a label to the keys of service instances that have that label value
type serviceInstanceValueSet map[string]serviceInstanceKeySet

func BuildServiceKey(r *Registry, s *model.Service) serviceKey {
	return serviceKey("[" + s.Hostname + "][" + string(r.Name) + "]")
}

func getIPHex(address string) string {
	ip := net.ParseIP(address)
	return hex.EncodeToString(ip)
}

func getPortHex(port int) string {
	pb := []byte{byte((port >> 8) & 0xFF), byte(port & 0xFF)}
	return hex.EncodeToString(pb)
}

func BuildServiceInstanceKey(i *model.ServiceInstance) serviceInstanceKey {
	return serviceInstanceKey("[" + i.Service.Hostname + "][" + getIPHex(i.Endpoint.Address) + "][" + getPortHex(i.Endpoint.Port) + "]")
}

type RegistryInstanceHandler struct {
	controller *Controller
	registry   *Registry
}

func BuildRegistryInstanceHandler(controller *Controller, registry *Registry) RegistryInstanceHandler {
	return RegistryInstanceHandler{controller, registry}
}

func (h *RegistryInstanceHandler) HandleService(s *model.Service, e model.Event) {
	k := BuildServiceKey(h.registry, s)
	h.controller.mu.Lock()
	defer h.controller.mu.Unlock()
	oldService, found := h.controller.services[k]
	if found {
		h.deleteService(k, oldService)
	}
	if e != model.EventDelete {
		// Treat as upsert
		h.addService(k, s)
	}
}

func (h *RegistryInstanceHandler) HandleServiceInstance(i *model.ServiceInstance, e model.Event) {
	k := BuildServiceInstanceKey(i)
	h.controller.mu.Lock()
	defer h.controller.mu.Unlock()
	oldInstance, found := h.controller.serviceInstances[k]
	if found {
		h.deleteServiceInstance(k, oldInstance)
	}
	if e != model.EventDelete {
		// Treat as upsert
		h.addServiceInstance(k, i)
	}
}

func (h *RegistryInstanceHandler) addService(k serviceKey, s *model.Service) {
	h.addServiceLabel(k, labelServiceName, s.Hostname)
	if s.ExternalName != "" {
		h.addServiceLabel(k, labelServiceDNS, s.ExternalName)
	}
	if s.Address != "" {
		h.addServiceLabel(k, labelServiceVIP, getIPHex(s.Address))
	}
	h.controller.services[k] = s
}

func (h *RegistryInstanceHandler) deleteService(k serviceKey, old *model.Service) {
	h.deleteServiceLabel(k, labelServiceName, old.Hostname)
	if old.ExternalName != "" {
		h.deleteServiceLabel(k, labelServiceDNS, old.ExternalName)
	}
	if old.Address != "" {
		h.deleteServiceLabel(k, labelServiceVIP, getIPHex(old.Address))
	}
	delete(h.controller.services, k)
}

func (h *RegistryInstanceHandler) addServiceLabel(k serviceKey, labelName, labelValue string) {
	valueKeySetMap, labelNameFound := h.controller.serviceLabels[labelName]
	if !labelNameFound {
		valueKeySetMap = make(serviceValueSet)
		h.controller.serviceLabels[labelName] = valueKeySetMap
	}
	keySet, labelValueFound := valueKeySetMap[labelValue]
	if !labelValueFound {
		keySet := make(serviceKeySet)
		valueKeySetMap[labelValue] = keySet
	}
	keySet[k] = true
}

func (h *RegistryInstanceHandler) deleteServiceLabel(k serviceKey, labelName, labelValue string) {
	valueKeySetMap, labelNameFound := h.controller.serviceLabels[labelName]
	if !labelNameFound {
		return
	}
	keySet, labelValueFound := valueKeySetMap[labelValue]
	if !labelValueFound {
		return
	}
	if keySet != nil {
		delete(keySet, k)
	}
}

func (h *RegistryInstanceHandler) addServiceInstance(k serviceInstanceKey, i *model.ServiceInstance) {
	h.addServiceInstanceLabel(k, labelInstanceService, i.Service.Hostname)
	h.addServiceInstanceLabel(k, labelInstanceIP, getIPHex(i.Endpoint.Address))
	h.addServiceInstanceLabel(k, labelInstancePort, getPortHex(i.Endpoint.Port))
	if i.Endpoint.ServicePort.Name != "" {
		h.addServiceInstanceLabel(k, labelInstanceNamedPort, i.Endpoint.ServicePort.Name)
	}
	for label, value := range i.Labels {
		h.addServiceInstanceLabel(k, label, value)
	}
	h.controller.serviceInstances[k] = i
}

func (h *RegistryInstanceHandler) deleteServiceInstance(k serviceInstanceKey, old *model.ServiceInstance) {
	h.deleteServiceInstanceLabel(k, labelInstanceService, old.Service.Hostname)
	h.deleteServiceInstanceLabel(k, labelInstanceIP, getIPHex(old.Endpoint.Address))
	h.deleteServiceInstanceLabel(k, labelInstancePort, getPortHex(old.Endpoint.Port))
	if old.Endpoint.ServicePort.Name != "" {
		h.deleteServiceInstanceLabel(k, labelInstanceNamedPort, old.Endpoint.ServicePort.Name)
	}
	for label, value := range old.Labels {
		h.deleteServiceInstanceLabel(k, label, value)
	}
	delete(h.controller.serviceInstances, k)
}

func (h *RegistryInstanceHandler) addServiceInstanceLabel(k serviceInstanceKey, labelName, labelValue string) {
	valueKeySetMap, labelNameFound := h.controller.serviceInstancesLabels[labelName]
	if !labelNameFound {
		valueKeySetMap = make(serviceInstanceValueSet)
		h.controller.serviceInstancesLabels[labelName] = valueKeySetMap
	}
	keySet, labelValueFound := valueKeySetMap[labelValue]
	if !labelValueFound {
		keySet := make(serviceInstanceKeySet)
		valueKeySetMap[labelValue] = keySet
	}
	keySet[k] = true
}

func (h *RegistryInstanceHandler) deleteServiceInstanceLabel(k serviceInstanceKey, labelName, labelValue string) {
	valueKeySetMap, labelNameFound := h.controller.serviceInstancesLabels[labelName]
	if !labelNameFound {
		return
	}
	keySet, labelValueFound := valueKeySetMap[labelValue]
	if !labelValueFound {
		return
	}
	if keySet != nil {
		delete(keySet, k)
	}
}
