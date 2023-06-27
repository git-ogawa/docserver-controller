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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DocServerSpec defines the desired state of DocServer
type DocServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Target is the properties used when pull the source of the document from a git repository.
	// +kubebuilder:validation:Required
	Target Target `json:"target,omitempty"`

	// Replicas is the number of docserver pod.
	// +kubebuilder:default=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the name:tag of the image used by the docserver container.
	// +optional
	Image string `json:"image,omitempty"`

	// Storage is the properties of persistenVolumeClaim.
	// +optional
	Storage Storage `json:"storage,omitempty"`

	// Gitpod is the properties of gitpod pods.
	// +optional
	Gitpod Gitpod `json:"gitpod,omitempty"`
}

type Target struct {
	// Url is the url of git repository where the sources of the document are stored.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^(https|ssh).*\.git$`
	Url string `json:"url"`

	// Branch is the branch name to be pulled.
	// +kubebuilder:default=main
	// +optional
	Branch string `json:"branch,omitempty"`

	// SSLVerify is the flag whether or not to check host identify when pull the source from the repository.
	// +optional
	SSLVerify *bool `json:"sslVerify,omitempty"`

	// BasicAuthSecret is the name of secret used when try basic authentication to pull the sources from the repository.
	// +optional
	BasicAuthSecret string `json:"basicAuthSecret,omitempty"`

	// SSHSecret is the name of secret used when using basic authentication to pull the sources from the repository.
	// +optional
	SSHSecret *SSHSecret `json:"sshSecret,omitempty"`

	// TLSSecret is the name of secret used when using try tls to pull the sources from the repository.
	// +optional
	TLSSecret string `json:"tlsSecret,omitempty"`

	// Depth is the depth to create shallow clone.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimm=1
	Depth int `json:"depth,omitempty"`
}

type SSHSecret struct {
	// Config is the name of configmap where ssh config is stored,
	// +optional
	Config string `json:"config,omitempty"`

	// PrivateKey is the name of secret where ssh private-key is stored.
	// +optional
	PrivateKey string `json:"privatekey,omitempty"`
}

type Storage struct {
	// Size is the volume capacity requested by persistenVolumeClaim.
	// +optional
	Size string `json:"size,omitempty"`

	// StorageClass is StorageClassName of persistenVolumeClaim.
	// +optional
	// +kubebuilder:default=default
	StorageClass string `json:"storageClass,omitempty"`

	// BlockOwnerDeletion is the value of BlockOwnerDeletion of persistenVolumeClaim.
	// +optional
	BlockOwnerDeletion *bool `json:"blockOwnerDeletion,omitempty"`
}

// Gitpod defines properties gitpod pods.
type Gitpod struct {
	// Image is the name:tag of the image used by the gitpod container.
	// +optional
	Image string `json:"image,omitempty"`
}

// DocServerStatus defines the observed state of DocServer
// type DocServerStatus struct {
// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
// Important: Run "make" to regenerate code after modifying this file
// }

// DocServerStatus defines the observed state of DocServer
// +kubebuilder:validation:Enum=NotReady;Available;Healthy
type DocServerStatus string

const (
	DocServerNotReady  = DocServerStatus("NotReady")
	DocServerAvailable = DocServerStatus("Available")
	DocServerHealthy   = DocServerStatus("Healthy")
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="REPLICAS",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="BRANCH",type="string",JSONPath=".spec.target.branch",priority=1
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.target.url",priority=1

// DocServer is the Schema for the docservers API
type DocServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DocServerSpec   `json:"spec,omitempty"`
	Status DocServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DocServerList contains a list of DocServer
type DocServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DocServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DocServer{}, &DocServerList{})
}
