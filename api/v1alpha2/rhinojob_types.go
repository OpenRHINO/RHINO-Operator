/*
Copyright 2022.

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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RhinoJobSpec defines the desired state of RhinoJob
type RhinoJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Image string `json:"image"`
	// +optional
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:default:=1
	Parallelism *int32 `json:"parallelism,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:default:=600
	TTL        *int32   `json:"ttl,omitempty"`
	AppExec    string   `json:"appExec"`
	AppArgs    []string `json:"appArgs,omitempty"`
	DataServer string   `json:"dataServer,omitempty"`
	DataPath   string   `json:"dataPath,omitempty"`
	// +optional
	// +kubebuilder:default:=FixedPerCoreMemory
	// +kubebuilder:validation:Enum=FixedTotalMemory;FixedPerCoreMemory
	MemoryAllocationMode MemoryAllocationMode `json:"memoryAllocationMode,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:default:=2
	MemoryAllocationSize *int32 `json:"memoryAllocationSize,omitempty"`
}

type MemoryAllocationMode string

const (
	FixedTotalMemory   MemoryAllocationMode = "FixedTotalMemory"
	FixedPerCoreMemory MemoryAllocationMode = "FixedPerCoreMemory"
)

// RhinoJobStatus defines the observed state of RhinoJob
type RhinoJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	JobStatus JobStatus `json:"jobStatus"`
}

type JobStatus string

const (
	Pending    JobStatus = "Pending"
	Running    JobStatus = "Running"
	Failed     JobStatus = "Failed"
	Completed  JobStatus = "Completed"
	ImageError JobStatus = "ImageError"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:name="Parallelism",type=integer,JSONPath=`.spec.parallelism`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.jobStatus`
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// RhinoJob is the Schema for the rhinojobs API
type RhinoJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RhinoJobSpec   `json:"spec,omitempty"`
	Status RhinoJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RhinoJobList contains a list of RhinoJob
type RhinoJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RhinoJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RhinoJob{}, &RhinoJobList{})
}
