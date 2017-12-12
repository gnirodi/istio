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
//	"errors"
//	"fmt"
	"testing"

	"istio.io/istio/pilot/model"
	"istio.io/istio/pilot/platform"
	"istio.io/istio/pilot/test/mock"
)

var platform1 platform.ServiceRegistry
var platform2 platform.ServiceRegistry
var discovery1 *mock.ServiceDiscovery
var discovery2 *mock.ServiceDiscovery
var evVerifier eventVerifier

// MockMeshResourceView specifies a mock MeshResourceView for testing
type MockController struct{
  model.ServiceDiscovery
  model.ServiceAccounts
  platform platform.ServiceRegistry 
}

type serviceEvent struct {
    *model.Service
    model.Event
}

type instanceEvent struct {
    *model.ServiceInstance
    model.Event
}

type eventVerifier struct {
    mockSvcHandlerMap map[platform.ServiceRegistry]func(*model.Service, model.Event)    
    svcTracked []serviceEvent
    svcToVerify []serviceEvent

    mockInstHandlerMap map[platform.ServiceRegistry]func(*model.ServiceInstance, model.Event)
    instTracked []instanceEvent
    instToVerify []instanceEvent
}

func newEventVerifier() *eventVerifier {
    out := eventVerifier{
        mockSvcHandlerMap: map[platform.ServiceRegistry]func(*model.Service, model.Event){},    
        svcTracked: []serviceEvent{},
        svcToVerify: []serviceEvent{},
        mockInstHandlerMap: map[platform.ServiceRegistry]func(*model.ServiceInstance, model.Event){},
        instTracked: []instanceEvent{},
        instToVerify: []instanceEvent{},
    }
    return &out
}

func (ev *eventVerifier) trackService(s *model.Service, e model.Event) {
    ev.svcTracked = append(ev.svcTracked, serviceEvent{s, e})
}

func (ev *eventVerifier) trackInstance(i *model.ServiceInstance, e model.Event) {
    ev.instTracked = append(ev.instTracked, instanceEvent{i, e})
}

func (ev *eventVerifier) mockSvcEvent(platform platform.ServiceRegistry, s *model.Service, e model.Event) {
    ev.svcToVerify = append(ev.svcToVerify, serviceEvent{s, e})
    handler, found := ev.mockSvcHandlerMap[platform]
    if found {
        handler(s, e)
    }
} 

func (ev *eventVerifier) mockInstEvent(platform platform.ServiceRegistry, i *model.ServiceInstance, e model.Event) {
    ev.instToVerify = append(ev.instToVerify, instanceEvent{i, e})
    handler, found := ev.mockInstHandlerMap[platform]
    if found {
        handler(i, e)
    }
}

func (ev *eventVerifier) verifyServiceEvents(t *testing.T) {
    expCount := len(ev.svcToVerify)
    actCount := len(ev.svcTracked)
    expIdx := 0
    for ; expIdx < expCount && expIdx < actCount; expIdx++ {
        expEvent := ev.svcToVerify[expIdx]
        actEvent := ev.svcTracked[expIdx]
        if expEvent.Service.Hostname != actEvent.Service.Hostname || expEvent.Event != actEvent.Event {
           t.Errorf("Unexpected out-of-sequence service event: expected event '%v', actual event '%v'", expEvent, actEvent)
        }
    }
    for ; expIdx < expCount; expIdx++ {
        expEvent := ev.svcToVerify[expIdx]
        t.Errorf("Expected service event: '%v', none actually tracked", expEvent)
    }
    for ; expIdx < actCount; expIdx++ {
        actEvent := ev.svcTracked[expIdx]
        t.Errorf("Unexpected extra service event: '%v'", actEvent)
    }
    ev.svcToVerify = []serviceEvent{}
    ev.svcTracked = []serviceEvent{}
}

func (ev *eventVerifier) verifyInstanceEvents(t *testing.T) {
    expCount := len(ev.instToVerify)
    actCount := len(ev.instTracked)
    expIdx := 0
    for ; expIdx < expCount && expIdx < actCount; expIdx++ {
        expEvent := ev.instToVerify[expIdx]
        actEvent := ev.instTracked[expIdx]
        if expEvent.ServiceInstance.Service.Hostname != actEvent.ServiceInstance.Service.Hostname || 
            expEvent.ServiceInstance.Endpoint.Address != actEvent.ServiceInstance.Endpoint.Address ||
            expEvent.ServiceInstance.Endpoint.Port != actEvent.ServiceInstance.Endpoint.Port ||
            expEvent.Event != actEvent.Event {
           t.Errorf("Unexpected out-of-sequence instance event: expected event '%v', actual event '%v'", expEvent, actEvent)
        }
    }
    for ; expIdx < expCount; expIdx++ {
        expEvent := ev.svcToVerify[expIdx]
        t.Errorf("Expected instance event: '%v', none actually tracked", expEvent)
    }
    for ; expIdx < actCount; expIdx++ {
        actEvent := ev.svcTracked[expIdx]
        t.Errorf("Unexpected extra instance event: '%v'", actEvent)
    }
    ev.instToVerify = []instanceEvent{}
    ev.instTracked = []instanceEvent{}
}

func (v *MockController) AppendServiceHandler(f func(*model.Service, model.Event)) error {
    evVerifier.mockSvcHandlerMap[v.platform] = f
	return nil
}

func (v *MockController) AppendInstanceHandler(f func(*model.ServiceInstance, model.Event)) error {
    evVerifier.mockInstHandlerMap[v.platform] = f
	return nil
}

func (v *MockController) Run(<-chan struct{}) {}

func buildMockMeshResourceView() *MeshResourceView {
	evVerifier = *newEventVerifier()
	discovery1 = mock.NewDiscovery(map[string]*model.Service{}, 0)
	discovery2 = mock.NewDiscovery(map[string]*model.Service{}, 0)
	platform1 = platform.ServiceRegistry("mockAdapter1")
	platform2 = platform.ServiceRegistry("mockAdapter2")

	registry1 := Registry{
		Name:             platform1,
		ServiceDiscovery: discovery1,
		ServiceAccounts:  discovery1,
		Controller: 	  &MockController{discovery1, discovery1, platform1},
	}

	registry2 := Registry{
		Name:             platform2,
		ServiceDiscovery: discovery2,
		ServiceAccounts:  discovery2,
		Controller: 	  &MockController{discovery2, discovery2, platform2},
	}

	meshView := NewMeshResourceView()
	meshView.AddRegistry(registry1)
	meshView.AddRegistry(registry2)
	meshView.AppendServiceHandler(evVerifier.trackService)
	meshView.AppendInstanceHandler(evVerifier.trackInstance)
	return meshView
}

func expectServices(t *testing.T, expected, actual []*model.Service) {
    expectedSet := map[string]int{}
    for _, svc := range expected {
        currentCount := expectedSet[svc.Hostname]
        expectedSet[svc.Hostname] = currentCount+1
    }
    actualSet := map[string]int{}
    for _, svc := range actual {
        currentCount := actualSet[svc.Hostname]
        actualSet[svc.Hostname] = currentCount+1
    }
    for svcName, countExpected := range expectedSet {
        countActual := actualSet[svcName]
        if countExpected != countActual {
           t.Errorf("Incorrect service count in mesh view. Expected '%d', Actual '%d', Service Name '%s'", countExpected, countActual, svcName) 
        }
        delete(actualSet, svcName)
    }
    if len(actualSet) > 0 {
       for svcName, countActual := range actualSet {
           t.Errorf("Unexpected service in mesh view. Expected '0', Actual '%d', Service Name '%s'", countActual, svcName) 
       }
    }
}

func svcList(s ...*model.Service) []*model.Service {
    return s
}

func TestServices(t *testing.T) {
	meshView := buildMockMeshResourceView()

	t.Run("EmptyMesh", func(t *testing.T) {
        	svcs, err := meshView.Services()
        	if err != nil {
        		t.Fatalf("Services() encountered unexpected error: %v", err)
        	}
            expectServices(t, svcList(), svcs)
            evVerifier.verifyServiceEvents(t)
	})

	evVerifier.mockSvcEvent(platform1, mock.HelloService, model.EventAdd)
	t.Run("AddHelloServicePlatform1", func(t *testing.T) {
        	svcs, err := meshView.Services()
        	if err != nil {
        		t.Fatalf("Services() encountered unexpected error: %v", err)
        	}
            expectServices(t, svcList(mock.HelloService), svcs)
            evVerifier.verifyServiceEvents(t)
	})
	
	evVerifier.mockSvcEvent(platform2, mock.WorldService, model.EventAdd)
	t.Run("AddWorldServicePlatform2", func(t *testing.T) {
        	svcs, err := meshView.Services()
        	if err != nil {
        		t.Fatalf("Services() encountered unexpected error: %v", err)
        	}
            expectServices(t, svcList(mock.HelloService, mock.WorldService), svcs)
            evVerifier.verifyServiceEvents(t)
	})
	
	evVerifier.mockSvcEvent(platform1, mock.WorldService, model.EventAdd)
	t.Run("AddWorldServiceAgainButPlatform1", func(t *testing.T) {
        	svcs, err := meshView.Services()
        	if err != nil {
        		t.Fatalf("Services() encountered unexpected error: %v", err)
        	}
            expectServices(t, svcList(mock.HelloService, mock.WorldService, mock.WorldService), svcs)
            evVerifier.verifyServiceEvents(t)
    })
	
	evVerifier.mockSvcEvent(platform2, mock.WorldService, model.EventDelete)
	t.Run("DeleteWorldServicePlatform2", func(t *testing.T) {
        	svcs, err := meshView.Services()
        	if err != nil {
        		t.Fatalf("Services() encountered unexpected error: %v", err)
        	}
            expectServices(t, svcList(mock.HelloService, mock.WorldService), svcs)
            evVerifier.verifyServiceEvents(t)
	})
	
	evVerifier.mockSvcEvent(platform1, mock.WorldService, model.EventUpdate)
	t.Run("UpdateWorldServicePlatform1", func(t *testing.T) {
        	svcs, err := meshView.Services()
        	if err != nil {
        		t.Fatalf("Services() encountered unexpected error: %v", err)
        	}
            expectServices(t, svcList(mock.HelloService, mock.WorldService), svcs)
            evVerifier.verifyServiceEvents(t)
	})
	
	evVerifier.mockSvcEvent(platform2, mock.WorldService, model.EventUpdate)
	t.Run("UpsertWorldServicePlatform2", func(t *testing.T) {
        	svcs, err := meshView.Services()
        	if err != nil {
        		t.Fatalf("Services() encountered unexpected error: %v", err)
        	}
            expectServices(t, svcList(mock.HelloService, mock.WorldService, mock.WorldService), svcs)
            evVerifier.verifyServiceEvents(t)
	})
	
	evVerifier.mockSvcEvent(platform1, mock.WorldService, model.EventDelete)
	evVerifier.mockSvcEvent(platform2, mock.WorldService, model.EventDelete)
	evVerifier.mockSvcEvent(platform1, mock.HelloService, model.EventDelete)
	t.Run("DeleteAll", func(t *testing.T) {
        	svcs, err := meshView.Services()
        	if err != nil {
        		t.Fatalf("Services() encountered unexpected error: %v", err)
        	}
            expectServices(t, svcList(), svcs)
            evVerifier.verifyServiceEvents(t)
	})
}


/*
func TestGetService(t *testing.T) {
	meshView := buildMockMeshResourceView()

	// Get service from mockAdapter1
	svc, err := meshView.GetService(mock.HelloService.Hostname)
	if err != nil {
		t.Fatalf("GetService() encountered unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("Fail to get service")
	}
	if svc.Hostname != mock.HelloService.Hostname {
		t.Fatal("Returned service is incorrect")
	}

	// Get service from mockAdapter2
	svc, err = meshView.GetService(mock.WorldService.Hostname)
	if err != nil {
		t.Fatalf("GetService() encountered unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("Fail to get service")
	}
	if svc.Hostname != mock.WorldService.Hostname {
		t.Fatal("Returned service is incorrect")
	}
}

func TestGetServiceError(t *testing.T) {
	meshView := buildMockMeshResourceView()

	// Get service from client with error
	svc, err := meshView.GetService(mock.HelloService.Hostname)
	if svc != nil {
		t.Fatal("GetService() should return nil if no service found")
	}

	// Get service from client without error
	svc, err = meshView.GetService(mock.WorldService.Hostname)
	if err != nil {
		t.Fatal("Aggregate MeshView should not return error if service is found")
	}
	if svc == nil {
		t.Fatal("Fail to get service")
	}
	if svc.Hostname != mock.WorldService.Hostname {
		t.Fatal("Returned service is incorrect")
	}
}

func TestHostInstances(t *testing.T) {
	meshView := buildMockMeshResourceView()

	// Get Instances from mockAdapter1
	instances, err := meshView.HostInstances(map[string]bool{mock.HelloInstanceV0: true})
	if err != nil {
		t.Fatalf("HostInstances() encountered unexpected error: %v", err)
	}
	if len(instances) != 5 {
		t.Fatalf("Returned HostInstances' amount %d is not correct", len(instances))
	}
	for _, inst := range instances {
		if inst.Service.Hostname != mock.HelloService.Hostname {
			t.Fatal("Returned Instance is incorrect")
		}
	}

	// Get Instances from mockAdapter2
	instances, err = meshView.HostInstances(map[string]bool{mock.MakeIP(mock.WorldService, 1): true})
	if err != nil {
		t.Fatalf("HostInstances() encountered unexpected error: %v", err)
	}
	if len(instances) != 5 {
		t.Fatalf("Returned HostInstances' amount %d is not correct", len(instances))
	}
	for _, inst := range instances {
		if inst.Service.Hostname != mock.WorldService.Hostname {
			t.Fatal("Returned Instance is incorrect")
		}
	}
}

func TestHostInstancesError(t *testing.T) {
	meshView := buildMockMeshResourceView()

	discovery1.HostInstancesError = errors.New("mock HostInstances() error")

	// Get Instances from client with error
	instances, err := meshView.HostInstances(map[string]bool{mock.HelloInstanceV0: true})
	if err == nil {
		t.Fatal("Aggregate MeshView should return error if one discovery client experiences " +
			"error and no instances are found")
	}
	if len(instances) != 0 {
		t.Fatal("HostInstances() should return no instances is client experiences error")
	}

	// Get Instances from client without error
	instances, err = meshView.HostInstances(map[string]bool{mock.MakeIP(mock.WorldService, 1): true})
	if err != nil {
		t.Fatal("Aggregate MeshView should not return error if instances are found")
	}
	if len(instances) != 5 {
		t.Fatalf("Returned HostInstances' amount %d is not correct", len(instances))
	}
	for _, inst := range instances {
		if inst.Service.Hostname != mock.WorldService.Hostname {
			t.Fatal("Returned Instance is incorrect")
		}
	}
}

func TestInstances(t *testing.T) {
	meshView := buildMockMeshResourceView()

	// Get Instances from mockAdapter1
	instances, err := meshView.Instances(mock.HelloService.Hostname,
		[]string{mock.PortHTTP.Name},
		model.LabelsCollection{})
	if err != nil {
		t.Fatalf("Instances() encountered unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatal("Returned wrong number of instances from MeshView")
	}
	for _, instance := range instances {
		if instance.Service.Hostname != mock.HelloService.Hostname {
			t.Fatal("Returned instance's hostname does not match desired value")
		}
		if _, ok := instance.Service.Ports.Get(mock.PortHTTP.Name); !ok {
			t.Fatal("Returned instance does not contain desired port")
		}
	}

	// Get Instances from mockAdapter2
	instances, err = meshView.Instances(mock.WorldService.Hostname,
		[]string{mock.PortHTTP.Name},
		model.LabelsCollection{})
	if err != nil {
		t.Fatalf("Instances() encountered unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatal("Returned wrong number of instances from MeshView")
	}
	for _, instance := range instances {
		if instance.Service.Hostname != mock.WorldService.Hostname {
			t.Fatal("Returned instance's hostname does not match desired value")
		}
		if _, ok := instance.Service.Ports.Get(mock.PortHTTP.Name); !ok {
			t.Fatal("Returned instance does not contain desired port")
		}
	}
}

func TestInstancesError(t *testing.T) {
	meshView := buildMockMeshResourceView()

	// Get Instances from client without error
	instances, err := meshView.Instances(mock.WorldService.Hostname,
		[]string{mock.PortHTTP.Name},
		model.LabelsCollection{})
	if err != nil {
		t.Fatalf("Instances() should not return error is instances are found: %v", err)
	}
	if len(instances) != 2 {
		t.Fatal("Returned wrong number of instances from MeshView")
	}
	for _, instance := range instances {
		if instance.Service.Hostname != mock.WorldService.Hostname {
			t.Fatal("Returned instance's hostname does not match desired value")
		}
		if _, ok := instance.Service.Ports.Get(mock.PortHTTP.Name); !ok {
			t.Fatal("Returned instance does not contain desired port")
		}
	}
}

func TestGetIstioServiceAccounts(t *testing.T) {
	meshView := buildMockMeshResourceView()

	// Get accounts from mockAdapter1
	accounts := meshView.GetIstioServiceAccounts(mock.HelloService.Hostname, []string{})
	expected := []string{}

	if len(accounts) != len(expected) {
		t.Fatal("Incorrect account result returned")
	}

	for i := 0; i < len(accounts); i++ {
		if accounts[i] != expected[i] {
			t.Fatal("Returned account result does not match expected one")
		}
	}

	// Get accounts from mockAdapter2
	accounts = meshView.GetIstioServiceAccounts(mock.WorldService.Hostname, []string{})
	expected = []string{
		"spiffe://cluster.local/ns/default/sa/serviceaccount1",
		"spiffe://cluster.local/ns/default/sa/serviceaccount2",
	}

	if len(accounts) != len(expected) {
		t.Fatal("Incorrect account result returned")
	}

	for i := 0; i < len(accounts); i++ {
		if accounts[i] != expected[i] {
			t.Fatal("Returned account result does not match expected one")
		}
	}
}

func TestManagementPorts(t *testing.T) {
	meshView := buildMockMeshResourceView()
	expected := model.PortList{{
		Name:     "http",
		Port:     3333,
		Protocol: model.ProtocolHTTP,
	}, {
		Name:     "custom",
		Port:     9999,
		Protocol: model.ProtocolTCP,
	}}

	// Get management ports from mockAdapter1
	ports := meshView.ManagementPorts(mock.HelloInstanceV0)
	if len(ports) != 2 {
		t.Fatal("Returned wrong number of ports from MeshView")
	}
	for i := 0; i < len(ports); i++ {
		if ports[i].Name != expected[i].Name || ports[i].Port != expected[i].Port ||
			ports[i].Protocol != expected[i].Protocol {
			t.Fatal("Returned management ports result does not match expected one")
		}
	}

	// Get management ports from mockAdapter2
	ports = meshView.ManagementPorts(mock.MakeIP(mock.WorldService, 0))
	if len(ports) != len(expected) {
		t.Fatal("Returned wrong number of ports from MeshView")
	}
	for i := 0; i < len(ports); i++ {
		if ports[i].Name != expected[i].Name || ports[i].Port != expected[i].Port ||
			ports[i].Protocol != expected[i].Protocol {
			t.Fatal("Returned management ports result does not match expected one")
		}
	}
}
*/