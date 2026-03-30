package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// SakuraAuthPolicySpec defines the desired state of SakuraAuthPolicy.
type SakuraAuthPolicySpec struct {
	// TargetRefs are the resources this policy attaches to (Gateway or HTTPRoute).
	TargetRefs []gatewayv1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRefs"`

	// Authentication configures the authentication method.
	// +optional
	Authentication *AuthenticationSpec `json:"authentication,omitempty"`

	// Authorization configures ACL-based authorization.
	// +optional
	Authorization *AuthorizationSpec `json:"authorization,omitempty"`

	// CORS configures Cross-Origin Resource Sharing.
	// +optional
	CORS *CORSSpec `json:"cors,omitempty"`

	// IPRestriction configures IP-based access control.
	// +optional
	IPRestriction *IPRestrictionSpec `json:"ipRestriction,omitempty"`
}

// AuthenticationSpec configures authentication.
type AuthenticationSpec struct {
	// Type is the authentication type: "jwt", "basic", "hmac", or "oidc".
	Type string `json:"type"`

	// JWT configures JWT-specific settings.
	// +optional
	JWT *JWTSpec `json:"jwt,omitempty"`

	// OIDC configures OIDC-specific settings.
	// +optional
	OIDC *OIDCSpec `json:"oidc,omitempty"`

	// Users defines the list of users for jwt/basic/hmac authentication.
	// +optional
	Users []UserSpec `json:"users,omitempty"`
}

// JWTSpec configures JWT authentication.
type JWTSpec struct {
	// Algorithm is the JWT signing algorithm: "HS256", "HS384", "HS512".
	Algorithm string `json:"algorithm"`
}

// OIDCSpec configures OIDC authentication.
type OIDCSpec struct {
	// Issuer is the OIDC issuer URL.
	Issuer string `json:"issuer"`
	// ClientID is the OIDC client ID.
	ClientID string `json:"clientId"`
	// ClientSecretRef references a Secret containing the OIDC client secret.
	ClientSecretRef SecretReference `json:"clientSecretRef"`
	// Scopes are the OIDC scopes to request.
	// +optional
	Scopes []string `json:"scopes,omitempty"`
	// AuthenticationMethods are the OIDC auth methods: "authorizationCodeFlow", "accessToken".
	// +optional
	AuthenticationMethods []string `json:"authenticationMethods,omitempty"`
	// TokenAudiences are the accepted audience values for token validation.
	// +optional
	TokenAudiences []string `json:"tokenAudiences,omitempty"`
}

// UserSpec defines a user for authentication.
type UserSpec struct {
	// Name is the user name.
	Name string `json:"name"`
	// CredentialsRef references a Secret containing authentication credentials.
	CredentialsRef SecretReference `json:"credentialsRef"`
	// Groups are the groups this user belongs to.
	// +optional
	Groups []string `json:"groups,omitempty"`
}

// AuthorizationSpec configures ACL-based authorization.
type AuthorizationSpec struct {
	// Enabled controls whether authorization is enabled.
	Enabled bool `json:"enabled"`
	// AllowGroups are the groups allowed access.
	// +optional
	AllowGroups []string `json:"allowGroups,omitempty"`
}

// CORSSpec configures CORS.
type CORSSpec struct {
	// AllowOrigins specifies allowed origins.
	// +optional
	AllowOrigins string `json:"allowOrigins,omitempty"`
	// AllowMethods specifies allowed HTTP methods.
	// +optional
	AllowMethods []string `json:"allowMethods,omitempty"`
	// AllowHeaders specifies allowed request headers.
	// +optional
	AllowHeaders []string `json:"allowHeaders,omitempty"`
	// MaxAge specifies how long preflight results are cached (seconds).
	// +optional
	MaxAge *int `json:"maxAge,omitempty"`
}

// IPRestrictionSpec configures IP-based access control.
type IPRestrictionSpec struct {
	// Type is "allowIps" or "denyIps".
	Type string `json:"type"`
	// IPs are the IP addresses.
	IPs []string `json:"ips"`
}

// SakuraAuthPolicyStatus defines the observed state of SakuraAuthPolicy.
type SakuraAuthPolicyStatus struct {
	// Conditions describe the current conditions of the SakuraAuthPolicy.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SakuraAuthPolicy is the Schema for the sakuraauthpolicies API.
// It is a Policy Attachment resource that configures authentication, authorization,
// CORS, and IP restriction for Gateway or HTTPRoute resources.
type SakuraAuthPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SakuraAuthPolicySpec   `json:"spec,omitempty"`
	Status SakuraAuthPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SakuraAuthPolicyList contains a list of SakuraAuthPolicy.
type SakuraAuthPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SakuraAuthPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SakuraAuthPolicy{}, &SakuraAuthPolicyList{})
}
