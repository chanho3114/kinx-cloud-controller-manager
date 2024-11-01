/*
Copyright 2024.

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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KinxCloudControllerManagerSpec defines the desired state of KinxCloudControllerManager
type KinxCloudControllerManagerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ClusterName                 string `json:"cluster_name"`
	AuthURL                     string `json:"auth_url"`
	ApplicationCredentialID     string `json:"application_credential_id"`
	ApplicationCredentialSecret string `json:"application_credential_secret"`
	ProjectName                 string `json:"project_name"`
	UserDomainName              string `json:"user_domain_name"`
	UseOctavia                  bool   `json:"use_octavia"`
}

// KinxCloudControllerManagerStatus defines the observed state of KinxCloudControllerManager
type KinxCloudControllerManagerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []appsv1.DeploymentCondition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KinxCloudControllerManager is the Schema for the kinxcloudcontrollermanagers API
type KinxCloudControllerManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KinxCloudControllerManagerSpec   `json:"spec,omitempty"`
	Status KinxCloudControllerManagerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KinxCloudControllerManagerList contains a list of KinxCloudControllerManager
type KinxCloudControllerManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KinxCloudControllerManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KinxCloudControllerManager{}, &KinxCloudControllerManagerList{})
}
