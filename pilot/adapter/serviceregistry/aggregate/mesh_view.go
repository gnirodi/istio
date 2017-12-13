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
	"errors"
	"net"
	"sync"

	"github.com/golang/glog"

	"istio.io/istio/pilot/model"
	"istio.io/istio/pilot/platform"
)

const (
	// Labels used for xDS internals use only. These labels are not intended for
	// external visibility within xDS interfaces.
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

// Registry specifies the collection of service registry related interfaces
type Registry struct {
	Name platform.ServiceRegistry
	model.Controller
	model.ServiceDiscovery
	model.ServiceAccounts
	// The service mesh view that this registry belongs to
	MeshView *MeshResourceView
}

// MeshResourceView is an aggregated store for resources sourced from various registries
type MeshResourceView struct {
	registries []Registry

	// Mutex guards services, serviceInstances and serviceInstanceLabels
	mu sync.RWMutex

	// Canonical map of mesh service key to service references
	services map[resourceKey]*model.Service

	// Canonical map of mesh service instance keys to service instance references
	serviceInstances map[resourceKey]*model.ServiceInstance

	// A reverse map that associates label names to label values and their associated service resource keys
	serviceLabels nameValueKeysMap

	// A reverse map that associates label names to label values and their associated service instance resource keys
	serviceInstanceLabels nameValueKeysMap

	// Cache notifier for Service resources
	serviceHandler func(*model.Service, model.Event)

	// Cache notifier for Service Instance resources
	serviceInstanceHandler func(*model.ServiceInstance, model.Event)
}

// NewMeshResourceView creates an aggregated store for resources sourced from various registries
func NewMeshResourceView() *MeshResourceView {
	return &MeshResourceView{
		registries:            make([]Registry, 0),
		mu:                    sync.RWMutex{},
		services:              make(map[resourceKey]*model.Service),
		serviceInstances:      make(map[resourceKey]*model.ServiceInstance),
		serviceLabels:         make(nameValueKeysMap),
		serviceInstanceLabels: make(nameValueKeysMap),
	}
}

// A resource key representing a mesh service resource
// Format for service: [service name][platformRegistry]
// There can only be one service object per platformRegistry
// TODO: this will change with multi-cluster to be one service object per platformRegistry per cluster
func BuildServiceKey(r *Registry, s *model.Service) resourceKey {
	return resourceKey("[" + s.Hostname + "][" + string(r.Name) + "]")
}

// Format for service instance: [service name][hex value of IP address][hex value of port number]
// Within the mesh there can be exactly one endpoint for a service with the combination of
// IP address and port
func BuildServiceInstanceKey(i *model.ServiceInstance) resourceKey {
	return resourceKey("[" + i.Service.Hostname + "][" + getIPHex(i.Endpoint.Address) + "][" + getPortHex(i.Endpoint.Port) + "]")
}

func getIPHex(address string) string {
	ip := net.ParseIP(address)
	return hex.EncodeToString(ip)
}

func getPortHex(port int) string {
	pb := []byte{byte((port >> 8) & 0xFF), byte(port & 0xFF)}
	return hex.EncodeToString(pb)
}

func (r *Registry) HandleService(s *model.Service, e model.Event) {
	k := BuildServiceKey(r, s)
	r.MeshView.handleService(k, s, e)
}

func (r *Registry) HandleServiceInstance(i *model.ServiceInstance, e model.Event) {
	k := BuildServiceInstanceKey(i)
	r.MeshView.handleServiceInstance(k, i, e)
}

// AddRegistry adds registries into the aggregated MeshResourceView
func (v *MeshResourceView) AddRegistry(registry Registry) {
	// Create bidirectional associations between the MeshView and
	// the registry being added
	registry.MeshView = v
	v.registries = append(v.registries, registry)
}

// Services lists services from all platforms
func (v *MeshResourceView) Services() ([]*model.Service, error) {
	lbls := resourceLabelsForName(labelServiceName)
	return v.serviceByLabels(lbls), nil
}

// GetService retrieves a service by hostname if exists
func (v *MeshResourceView) GetService(hostname string) (*model.Service, error) {
	lbls := resourceLabelsForNameValue(labelServiceName, hostname)
	svcs := v.serviceByLabels(lbls)
	if len(svcs) > 0 {
		return svcs[0], nil
	}
	return nil, nil
}

// ManagementPorts retrieves set of health check ports by instance IP
// Return on the first hit.
func (v *MeshResourceView) ManagementPorts(addr string) model.PortList {
	for _, r := range v.registries {
		if portList := r.ManagementPorts(addr); portList != nil {
			return portList
		}
	}
	return nil
}

// Instances retrieves instances for a service and its ports that match
// any of the supplied labels. All instances match an empty label list.
func (v *MeshResourceView) Instances(hostname string, ports []string,
	labels model.LabelsCollection) ([]*model.ServiceInstance, error) {
	hostPortLbls := resourceLabelsForValues(labelInstancePort, ports)
	hostPortLbls.appendNameValue(labelInstanceService, hostname)
	if len(labels) > 0 {
		for _, lblset := range labels {
			lbls := resourceLabelsFromModelLabels(lblset)
			lbls = append(lbls, hostPortLbls...)
			out := v.serviceInstancesByLabels(lbls)
			if len(out) > 0 {
				return out, nil
			}
		}
		return nil, nil
	}

	return v.serviceInstancesByLabels(hostPortLbls), nil
}

// HostInstances lists service instances for a given set of IPv4 addresses.
func (v *MeshResourceView) HostInstances(addrs map[string]bool) ([]*model.ServiceInstance, error) {
	lbls := resourceLabelsForValueSet(labelInstanceIP, addrs)
	return v.serviceInstancesByLabels(lbls), nil
}

// Run starts all the MeshResourceViews
func (v *MeshResourceView) Run(stop <-chan struct{}) {

	for _, r := range v.registries {
		go r.Run(stop)
	}

	<-stop
	glog.V(2).Info("Registry Aggregator terminated")
}

// AppendServiceHandler implements a service catalog operation, but limits to only one handler
func (v *MeshResourceView) AppendServiceHandler(f func(*model.Service, model.Event)) error {
	if v.serviceHandler != nil {
		logMsg := "Fail to append service handler to aggregated mesh view. Maximum number of handlers '1' already added."
		glog.V(2).Info(logMsg)
		return errors.New(logMsg)
	}
	v.serviceHandler = f
	for idx := range v.registries {
		r := &v.registries[idx]
		if err := r.AppendServiceHandler(r.HandleService); err != nil {
			glog.V(2).Infof("Fail to append service handler to adapter %s", r.Name)
			return err
		}
	}
	return nil
}

// AppendInstanceHandler implements a service instance catalog operation, but limits to only one handler
func (v *MeshResourceView) AppendInstanceHandler(f func(*model.ServiceInstance, model.Event)) error {
	if v.serviceInstanceHandler != nil {
		logMsg := "Fail to append service instance handler to aggregated mesh view. Maximum number of handlers '1' already added."
		glog.V(2).Info(logMsg)
		return errors.New(logMsg)
	}
	v.serviceInstanceHandler = f
	for idx := range v.registries {
		r := &v.registries[idx]
		if err := r.AppendInstanceHandler(r.HandleServiceInstance); err != nil {
			glog.V(2).Infof("Fail to append instance handler to adapter %s", r.Name)
			return err
		}
	}
	return nil
}

// GetIstioServiceAccounts implements model.ServiceAccounts operation
func (v *MeshResourceView) GetIstioServiceAccounts(hostname string, ports []string) []string {
	hostLabel := resourceLabelsForNameValue(labelServiceName, hostname)
	hostPortLbls := resourceLabelsForValues(labelInstancePort, ports)
	hostPortLbls.appendFrom(hostLabel)
	instances := v.serviceInstancesByLabels(hostPortLbls)
	saSet := make(map[string]bool)
	for _, si := range instances {
		if si.ServiceAccount != "" {
			saSet[si.ServiceAccount] = true
		}
	}
	svcs := v.serviceByLabels(hostLabel)
	for _, svc := range svcs {
		for _, serviceAccount := range svc.ServiceAccounts {
			sa := serviceAccount
			saSet[sa] = true
		}
	}
	saArray := make([]string, 0, len(saSet))
	for sa := range saSet {
		saArray = append(saArray, sa)
	}
	return saArray
}

func (v *MeshResourceView) handleService(k resourceKey, s *model.Service, e model.Event) {
	v.mu.Lock()
	defer v.mu.Unlock()
	old, found := v.services[k]
	if found {
		v.serviceLabels.deleteLabel(k, labelServiceName, old.Hostname)
		if old.ExternalName != "" {
			v.serviceLabels.deleteLabel(k, labelServiceDNS, old.ExternalName)
		}
		if old.Address != "" {
			v.serviceLabels.deleteLabel(k, labelServiceVIP, getIPHex(old.Address))
		}
		delete(v.services, k)
	}
	if e != model.EventDelete {
		// Treat as upsert
		v.serviceLabels.addLabel(k, labelServiceName, s.Hostname)
		if s.ExternalName != "" {
			v.serviceLabels.addLabel(k, labelServiceDNS, s.ExternalName)
		}
		if s.Address != "" {
			v.serviceLabels.addLabel(k, labelServiceVIP, getIPHex(s.Address))
		}
		v.services[k] = s
	}
	v.serviceHandler(s, e)
}

func (v *MeshResourceView) handleServiceInstance(k resourceKey, i *model.ServiceInstance, e model.Event) {
	v.mu.Lock()
	defer v.mu.Unlock()
	old, found := v.serviceInstances[k]
	if found {
		v.serviceInstanceLabels.deleteLabel(k, labelInstanceService, old.Service.Hostname)
		v.serviceInstanceLabels.deleteLabel(k, labelInstanceIP, getIPHex(old.Endpoint.Address))
		v.serviceInstanceLabels.deleteLabel(k, labelInstancePort, getPortHex(old.Endpoint.Port))
		if old.Endpoint.ServicePort.Name != "" {
			v.serviceInstanceLabels.deleteLabel(k, labelInstanceNamedPort, old.Endpoint.ServicePort.Name)
		}
		for label, value := range old.Labels {
			v.serviceInstanceLabels.deleteLabel(k, label, value)
		}
		delete(v.serviceInstances, k)
	}
	if e != model.EventDelete {
		// Treat as upsert
		v.serviceInstanceLabels.addLabel(k, labelInstanceService, i.Service.Hostname)
		v.serviceInstanceLabels.addLabel(k, labelInstanceIP, getIPHex(i.Endpoint.Address))
		v.serviceInstanceLabels.addLabel(k, labelInstancePort, getPortHex(i.Endpoint.Port))
		if i.Endpoint.ServicePort.Name != "" {
			v.serviceInstanceLabels.addLabel(k, labelInstanceNamedPort, i.Endpoint.ServicePort.Name)
		}
		for label, value := range i.Labels {
			v.serviceInstanceLabels.addLabel(k, label, value)
		}
		v.serviceInstances[k] = i
	}
	v.serviceInstanceHandler(i, e)
}

func (v *MeshResourceView) serviceByLabels(labels resourceLabels) []*model.Service {
	v.mu.RLock()
	defer v.mu.RUnlock()
	svcKeySet := v.serviceLabels.getResourceKeysMatching(labels)
	out := make([]*model.Service, len(svcKeySet))
	i := 0
	for k := range svcKeySet {
		out[i] = v.services[k]
		i++
	}
	return out
}

func (v *MeshResourceView) serviceInstancesByLabels(labels resourceLabels) []*model.ServiceInstance {
	v.mu.RLock()
	defer v.mu.RUnlock()
	instanceKeySet := v.serviceInstanceLabels.getResourceKeysMatching(labels)
	out := make([]*model.ServiceInstance, len(instanceKeySet))
	i := 0
	for k := range instanceKeySet {
		out[i] = v.serviceInstances[k]
		i++
	}
	return out
}
