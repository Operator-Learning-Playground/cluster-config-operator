package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterConfig
type ClusterConfig struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterConfigSpec `json:"spec,omitempty"`
}

type ClusterConfigSpec struct {
	NamespaceList string            `json:"namespaceList,omitempty"`
	ConfigType    string            `json:"configType,omitempty"`
	Data          map[string]string `json:"data,omitempty"`
	Type          v1.SecretType     `json:"type,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterConfigList
type ClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ClusterConfig `json:"items"`
}
