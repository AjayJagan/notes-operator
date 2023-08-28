/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NotesAppSpec defines the desired state of NotesApp
type NotesAppSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of NotesApp. Edit notesapp_types.go to remove/update
	NotesAppFe   NotesAppFe   `json:"notesAppFe,omitempty"`
	NotesAppBe   NotesAppBe   `json:"notesAppBe,omitempty"`
	MongoDb      MongoDb      `json:"mongoDb,omitempty"`
	NginxIngress NginxIngress `json:"nginxIngress,omitempty"`
}

type NotesAppFe struct {
	Repository  string             `json:"repository,omitempty"`
	Tag         string             `json:"tag,omitempty"`
	Replicas    int32              `json:"replicas,omitempty"`
	Port        int32              `json:"port,omitempty"`
	TargetPort  int                `json:"targetPort,omitempty"`
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
}

type NotesAppBe struct {
	Repository  string             `json:"repository,omitempty"`
	Tag         string             `json:"tag,omitempty"`
	Replicas    int32              `json:"replicas,omitempty"`
	Port        int32              `json:"port,omitempty"`
	TargetPort  int                `json:"targetPort,omitempty"`
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
}

type MongoDb struct {
	Repository string            `json:"repository,omitempty"`
	Tag        string            `json:"tag,omitempty"`
	Replicas   int32             `json:"replicas,omitempty"`
	Port       int32             `json:"port,omitempty"`
	TargetPort int               `json:"targetPort,omitempty"`
	DBSize     resource.Quantity `json:"dbSize,omitempty"`
}

type NginxIngress struct {
	FePort int32 `json:"fePort,omitempty"`
	BePort int32 `json:"bePort,omitempty"`
}

// NotesAppStatus defines the observed state of NotesApp
type NotesAppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NotesApp is the Schema for the notesapps API
type NotesApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NotesAppSpec   `json:"spec,omitempty"`
	Status NotesAppStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NotesAppList contains a list of NotesApp
type NotesAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NotesApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NotesApp{}, &NotesAppList{})
}
