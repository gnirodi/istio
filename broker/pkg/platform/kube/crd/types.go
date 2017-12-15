// Code generated by generate.sh. DO NOT EDIT!
// Sources: pkg/model/config/resource.go
// Output: pkg/platform/kube/crd/types.go

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

package crd

// This file contains Go definitions for Custom Resource Definition kinds
// to adhere to the idiomatic use of k8s API machinery.
// These definitions are synthesized from Istio configuration type descriptors
// as declared in the Broker config model.

import (
	"istio.io/istio/broker/pkg/model/config"
	"istio.io/istio/broker/pkg/testing/mock"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var knownTypes = map[string]struct {
	object     IstioObject
	collection IstioObjectList
}{
	config.ServiceClass.Type: {
		object: &ServiceClass{
			TypeMeta: meta_v1.TypeMeta{
				Kind:       "ServiceClass",
				APIVersion: config.IstioAPIVersion,
			},
		},
		collection: &ServiceClassList{},
	},
	config.ServicePlan.Type: {
		object: &ServicePlan{
			TypeMeta: meta_v1.TypeMeta{
				Kind:       "ServicePlan",
				APIVersion: config.IstioAPIVersion,
			},
		},
		collection: &ServicePlanList{},
	},
	mock.FakeConfig.Type: {
		object: &FakeConfig{
			TypeMeta: meta_v1.TypeMeta{
				Kind:       "FakeConfig",
				APIVersion: config.IstioAPIVersion,
			},
		},
		collection: &FakeConfigList{},
	},
}

// ServiceClass is the generic Kubernetes API object wrapper
type ServiceClass struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               map[string]interface{} `json:"spec"`
}

// GetSpec from a wrapper
func (in *ServiceClass) GetSpec() map[string]interface{} {
	return in.Spec
}

// SetSpec for a wrapper
func (in *ServiceClass) SetSpec(spec map[string]interface{}) {
	in.Spec = spec
}

// GetObjectMeta from a wrapper
func (in *ServiceClass) GetObjectMeta() meta_v1.ObjectMeta {
	return in.ObjectMeta
}

// SetObjectMeta for a wrapper
func (in *ServiceClass) SetObjectMeta(metadata meta_v1.ObjectMeta) {
	in.ObjectMeta = metadata
}

// ServiceClassList is the generic Kubernetes API list wrapper
type ServiceClassList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []*ServiceClass `json:"items"`
}

// GetItems from a wrapper
func (in *ServiceClassList) GetItems() []IstioObject {
	out := make([]IstioObject, len(in.Items))
	for i, v := range in.Items {
		out[i] = v
	}
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceClass) DeepCopyInto(out *ServiceClass) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceClass.
func (in *ServiceClass) DeepCopy() *ServiceClass {
	if in == nil {
		return nil
	}
	out := new(ServiceClass)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceClass) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceClassList) DeepCopyInto(out *ServiceClassList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]*ServiceClass, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto((*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceClassList.
func (in *ServiceClassList) DeepCopy() *ServiceClassList {
	if in == nil {
		return nil
	}
	out := new(ServiceClassList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceClassList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

// ServicePlan is the generic Kubernetes API object wrapper
type ServicePlan struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               map[string]interface{} `json:"spec"`
}

// GetSpec from a wrapper
func (in *ServicePlan) GetSpec() map[string]interface{} {
	return in.Spec
}

// SetSpec for a wrapper
func (in *ServicePlan) SetSpec(spec map[string]interface{}) {
	in.Spec = spec
}

// GetObjectMeta from a wrapper
func (in *ServicePlan) GetObjectMeta() meta_v1.ObjectMeta {
	return in.ObjectMeta
}

// SetObjectMeta for a wrapper
func (in *ServicePlan) SetObjectMeta(metadata meta_v1.ObjectMeta) {
	in.ObjectMeta = metadata
}

// ServicePlanList is the generic Kubernetes API list wrapper
type ServicePlanList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []*ServicePlan `json:"items"`
}

// GetItems from a wrapper
func (in *ServicePlanList) GetItems() []IstioObject {
	out := make([]IstioObject, len(in.Items))
	for i, v := range in.Items {
		out[i] = v
	}
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServicePlan) DeepCopyInto(out *ServicePlan) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServicePlan.
func (in *ServicePlan) DeepCopy() *ServicePlan {
	if in == nil {
		return nil
	}
	out := new(ServicePlan)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServicePlan) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServicePlanList) DeepCopyInto(out *ServicePlanList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]*ServicePlan, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto((*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServicePlanList.
func (in *ServicePlanList) DeepCopy() *ServicePlanList {
	if in == nil {
		return nil
	}
	out := new(ServicePlanList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServicePlanList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

// FakeConfig is the generic Kubernetes API object wrapper
type FakeConfig struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               map[string]interface{} `json:"spec"`
}

// GetSpec from a wrapper
func (in *FakeConfig) GetSpec() map[string]interface{} {
	return in.Spec
}

// SetSpec for a wrapper
func (in *FakeConfig) SetSpec(spec map[string]interface{}) {
	in.Spec = spec
}

// GetObjectMeta from a wrapper
func (in *FakeConfig) GetObjectMeta() meta_v1.ObjectMeta {
	return in.ObjectMeta
}

// SetObjectMeta for a wrapper
func (in *FakeConfig) SetObjectMeta(metadata meta_v1.ObjectMeta) {
	in.ObjectMeta = metadata
}

// FakeConfigList is the generic Kubernetes API list wrapper
type FakeConfigList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []*FakeConfig `json:"items"`
}

// GetItems from a wrapper
func (in *FakeConfigList) GetItems() []IstioObject {
	out := make([]IstioObject, len(in.Items))
	for i, v := range in.Items {
		out[i] = v
	}
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FakeConfig) DeepCopyInto(out *FakeConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FakeConfig.
func (in *FakeConfig) DeepCopy() *FakeConfig {
	if in == nil {
		return nil
	}
	out := new(FakeConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FakeConfig) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FakeConfigList) DeepCopyInto(out *FakeConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]*FakeConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto((*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FakeConfigList.
func (in *FakeConfigList) DeepCopy() *FakeConfigList {
	if in == nil {
		return nil
	}
	out := new(FakeConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FakeConfigList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}
