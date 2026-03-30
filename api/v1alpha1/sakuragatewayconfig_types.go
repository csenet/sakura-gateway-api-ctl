package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SakuraGatewayConfigSpec defines the desired state of SakuraGatewayConfig.
type SakuraGatewayConfigSpec struct {
	// CredentialsRef is a reference to a Secret containing Sakura Cloud API credentials.
	// The Secret must contain "access-token" and "access-token-secret" keys.
	CredentialsRef SecretReference `json:"credentialsRef"`

	// Subscription configures the Sakura Cloud API Gateway subscription.
	Subscription SubscriptionSpec `json:"subscription"`

	// GatewayVerification configures shared secret header verification.
	// +optional
	GatewayVerification *GatewayVerificationSpec `json:"gatewayVerification,omitempty"`
}

// SecretReference is a reference to a Secret in a given namespace.
type SecretReference struct {
	// Name is the name of the Secret.
	Name string `json:"name"`
	// Namespace is the namespace of the Secret.
	Namespace string `json:"namespace"`
}

// SubscriptionSpec configures the Sakura Cloud API Gateway subscription.
type SubscriptionSpec struct {
	// ID is the existing subscription ID. If set, PlanID and Name are ignored.
	// +optional
	ID *string `json:"id,omitempty"`
	// PlanID is the plan ID for creating a new subscription.
	// +optional
	PlanID *string `json:"planId,omitempty"`
	// Name is the name for the new subscription.
	// +optional
	Name *string `json:"name,omitempty"`
}

// GatewayVerificationSpec configures shared secret header for bypass prevention.
type GatewayVerificationSpec struct {
	// Enabled controls whether gateway verification is enabled. Default: true.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// HeaderName is the header name for the shared secret. Default: "X-Gateway-Secret".
	// +optional
	HeaderName string `json:"headerName,omitempty"`
	// RotationInterval is the interval for secret rotation. Default: "24h".
	// +optional
	RotationInterval string `json:"rotationInterval,omitempty"`
}

// SakuraGatewayConfigStatus defines the observed state of SakuraGatewayConfig.
type SakuraGatewayConfigStatus struct {
	// SubscriptionID is the resolved subscription ID.
	// +optional
	SubscriptionID string `json:"subscriptionId,omitempty"`
	// PlanName is the name of the subscription plan.
	// +optional
	PlanName string `json:"planName,omitempty"`
	// MonthlyRequests is the current month's request count.
	// +optional
	MonthlyRequests int64 `json:"monthlyRequests,omitempty"`
	// Conditions describe the current conditions of the SakuraGatewayConfig.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Subscription",type="string",JSONPath=".status.subscriptionId"
// +kubebuilder:printcolumn:name="Plan",type="string",JSONPath=".status.planName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SakuraGatewayConfig is the Schema for the sakuragatewayconfigs API.
// It is referenced by GatewayClass parametersRef and holds Sakura Cloud API credentials and subscription info.
type SakuraGatewayConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SakuraGatewayConfigSpec   `json:"spec,omitempty"`
	Status SakuraGatewayConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SakuraGatewayConfigList contains a list of SakuraGatewayConfig.
type SakuraGatewayConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SakuraGatewayConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SakuraGatewayConfig{}, &SakuraGatewayConfigList{})
}

// IsVerificationEnabled returns whether gateway verification is enabled.
func (s *SakuraGatewayConfigSpec) IsVerificationEnabled() bool {
	if s.GatewayVerification == nil {
		return true // default enabled
	}
	if s.GatewayVerification.Enabled == nil {
		return true
	}
	return *s.GatewayVerification.Enabled
}

// GetVerificationHeaderName returns the header name for gateway verification.
func (s *SakuraGatewayConfigSpec) GetVerificationHeaderName() string {
	if s.GatewayVerification == nil || s.GatewayVerification.HeaderName == "" {
		return "X-Gateway-Secret"
	}
	return s.GatewayVerification.HeaderName
}
