package sakura

// apiResponse wraps all Sakura API responses in {"apigw": ...}.
type apiResponse[T any] struct {
	APIGW T `json:"apigw"`
}

// apiListResponse wraps list API responses in {"apigw": [...]}.
type apiListResponse[T any] struct {
	APIGW []T `json:"apigw"`
}

// Plan represents an API Gateway plan.
type Plan struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Price       int      `json:"price"`
	MaxServices int      `json:"maxServices"`
	MaxRequests int      `json:"maxRequests"`
	Overage     *Overage `json:"overage,omitempty"`
}

// Overage represents overage pricing.
type Overage struct {
	UnitRequests int `json:"unitRequests"`
	UnitPrice    int `json:"unitPrice"`
}

// Subscription represents an API Gateway subscription (contract).
type Subscription struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	ResourceID     int64   `json:"resourceId,omitempty"`
	MonthlyRequest int64   `json:"monthlyRequest"`
	Plan           *Plan   `json:"plan,omitempty"`
	CreatedAt      string  `json:"createdAt,omitempty"`
	UpdatedAt      string  `json:"updatedAt,omitempty"`
}

// CreateSubscriptionRequest is the request body for POST /subscriptions.
type CreateSubscriptionRequest struct {
	PlanID string `json:"planId"`
	Name   string `json:"name"`
}

// UpdateSubscriptionRequest is the request body for PUT /subscriptions/{id}.
type UpdateSubscriptionRequest struct {
	Name string `json:"name"`
}

// Service represents an API Gateway service (backend).
type Service struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Tags          []string    `json:"tags,omitempty"`
	Protocol      string      `json:"protocol"`
	Host          string      `json:"host"`
	Path          string      `json:"path,omitempty"`
	Port          *int        `json:"port,omitempty"`
	Retries       *int        `json:"retries,omitempty"`
	ConnectTimeout *int       `json:"connectTimeout,omitempty"`
	WriteTimeout  *int        `json:"writeTimeout,omitempty"`
	ReadTimeout   *int        `json:"readTimeout,omitempty"`
	Authentication string     `json:"authentication,omitempty"`
	OIDC          *OIDCRef    `json:"oidc,omitempty"`
	CorsConfig    *CorsConfig `json:"corsConfig,omitempty"`
	RouteHost     string      `json:"routeHost,omitempty"`
	Subscription  *SubscriptionRef `json:"subscription,omitempty"`
	CreatedAt     string      `json:"createdAt,omitempty"`
	UpdatedAt     string      `json:"updatedAt,omitempty"`
}

// SubscriptionRef is a reference to a subscription.
type SubscriptionRef struct {
	ID string `json:"id"`
}

// OIDCRef is a reference to an OIDC configuration on a service.
type OIDCRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// CorsConfig configures CORS on a service.
type CorsConfig struct {
	Credentials                 bool     `json:"credentials"`
	AccessControlAllowOrigins   string   `json:"accessControlAllowOrigins,omitempty"`
	AccessControlAllowMethods   []string `json:"accessControlAllowMethods,omitempty"`
	AccessControlAllowHeaders   string   `json:"accessControlAllowHeaders,omitempty"`
	AccessControlExposedHeaders string   `json:"accessControlExposedHeaders,omitempty"`
	MaxAge                      int      `json:"maxAge,omitempty"`
	PreflightContinue           bool     `json:"preflightContinue"`
	PrivateNetwork              bool     `json:"privateNetwork"`
}

// CreateServiceRequest is the request body for POST /services.
type CreateServiceRequest struct {
	Name           string          `json:"name"`
	Tags           []string        `json:"tags,omitempty"`
	Protocol       string          `json:"protocol"`
	Host           string          `json:"host"`
	Path           string          `json:"path,omitempty"`
	Port           *int            `json:"port,omitempty"`
	Authentication string          `json:"authentication,omitempty"`
	CorsConfig     *CorsConfig     `json:"corsConfig,omitempty"`
	Subscription   *SubscriptionRef `json:"subscription"`
}

// UpdateServiceRequest is the request body for PUT /services/{id}.
type UpdateServiceRequest struct {
	Name           string          `json:"name,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	Protocol       string          `json:"protocol,omitempty"`
	Host           string          `json:"host,omitempty"`
	Path           string          `json:"path,omitempty"`
	Port           *int            `json:"port,omitempty"`
	Authentication string          `json:"authentication,omitempty"`
	OIDC           *OIDCRef        `json:"oidc,omitempty"`
	CorsConfig     *CorsConfig     `json:"corsConfig,omitempty"`
	Subscription   *SubscriptionRef `json:"subscription,omitempty"`
}

// Route represents an API Gateway route.
type Route struct {
	ID                      string             `json:"id"`
	ServiceID               string             `json:"serviceId,omitempty"`
	Name                    string             `json:"name"`
	Tags                    []string           `json:"tags,omitempty"`
	Protocols               string             `json:"protocols"`
	Path                    string             `json:"path,omitempty"`
	Hosts                   []string           `json:"hosts,omitempty"`
	Methods                 []string           `json:"methods,omitempty"`
	HTTPSRedirectStatusCode *int               `json:"httpsRedirectStatusCode,omitempty"`
	RegexPriority           *int               `json:"regexPriority,omitempty"`
	StripPath               *bool              `json:"stripPath,omitempty"`
	PreserveHost            *bool              `json:"preserveHost,omitempty"`
	RequestBuffering        *bool              `json:"requestBuffering,omitempty"`
	ResponseBuffering       *bool              `json:"responseBuffering,omitempty"`
	IPRestrictionConfig     *IPRestrictionConfig `json:"ipRestrictionConfig,omitempty"`
	Host                    string             `json:"host,omitempty"`
	CreatedAt               string             `json:"createdAt,omitempty"`
	UpdatedAt               string             `json:"updatedAt,omitempty"`
}

// IPRestrictionConfig configures IP-based access control on a route.
type IPRestrictionConfig struct {
	Protocols    string   `json:"protocols,omitempty"`
	RestrictedBy string   `json:"restrictedBy"`
	IPs          []string `json:"ips"`
}

// CreateRouteRequest is the request body for POST /services/{serviceId}/routes.
type CreateRouteRequest struct {
	Name                    string             `json:"name"`
	Tags                    []string           `json:"tags,omitempty"`
	Protocols               string             `json:"protocols"`
	Path                    string             `json:"path,omitempty"`
	Hosts                   []string           `json:"hosts,omitempty"`
	Methods                 []string           `json:"methods,omitempty"`
	HTTPSRedirectStatusCode *int               `json:"httpsRedirectStatusCode,omitempty"`
	RegexPriority           *int               `json:"regexPriority,omitempty"`
	StripPath               *bool              `json:"stripPath,omitempty"`
	PreserveHost            *bool              `json:"preserveHost,omitempty"`
	IPRestrictionConfig     *IPRestrictionConfig `json:"ipRestrictionConfig,omitempty"`
}

// UpdateRouteRequest is the request body for PUT /services/{serviceId}/routes/{routeId}.
type UpdateRouteRequest struct {
	Name                    string             `json:"name,omitempty"`
	Tags                    []string           `json:"tags,omitempty"`
	Protocols               string             `json:"protocols,omitempty"`
	Path                    string             `json:"path,omitempty"`
	Hosts                   []string           `json:"hosts,omitempty"`
	Methods                 []string           `json:"methods,omitempty"`
	HTTPSRedirectStatusCode *int               `json:"httpsRedirectStatusCode,omitempty"`
	StripPath               *bool              `json:"stripPath,omitempty"`
	PreserveHost            *bool              `json:"preserveHost,omitempty"`
	IPRestrictionConfig     *IPRestrictionConfig `json:"ipRestrictionConfig,omitempty"`
}

// HeaderKeyValue represents a key-value pair for header manipulation.
type HeaderKeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// RenameKeyValue represents a from-to pair for renaming.
type RenameKeyValue struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RequestTransform represents request transformation settings.
type RequestTransform struct {
	HTTPMethod string               `json:"httpMethod,omitempty"`
	Allow      *RequestTransformAllow  `json:"allow,omitempty"`
	Remove     *RequestTransformRemove `json:"remove,omitempty"`
	Rename     *RequestTransformRename `json:"rename,omitempty"`
	Replace    *RequestTransformReplace `json:"replace,omitempty"`
	Add        *RequestTransformAdd    `json:"add,omitempty"`
	Append     *RequestTransformAppend `json:"append,omitempty"`
}

type RequestTransformAllow struct {
	Body []string `json:"body,omitempty"`
}

type RequestTransformRemove struct {
	HeaderKeys  []string `json:"headerKeys,omitempty"`
	QueryParams []string `json:"queryParams,omitempty"`
	Body        []string `json:"body,omitempty"`
}

type RequestTransformRename struct {
	Headers     []RenameKeyValue `json:"headers,omitempty"`
	QueryParams []RenameKeyValue `json:"queryParams,omitempty"`
	Body        []RenameKeyValue `json:"body,omitempty"`
}

type RequestTransformReplace struct {
	Headers     []HeaderKeyValue `json:"headers,omitempty"`
	QueryParams []HeaderKeyValue `json:"queryParams,omitempty"`
	Body        []HeaderKeyValue `json:"body,omitempty"`
}

type RequestTransformAdd struct {
	Headers     []HeaderKeyValue `json:"headers,omitempty"`
	QueryParams []HeaderKeyValue `json:"queryParams,omitempty"`
	Body        []HeaderKeyValue `json:"body,omitempty"`
}

type RequestTransformAppend struct {
	Headers     []HeaderKeyValue `json:"headers,omitempty"`
	QueryParams []HeaderKeyValue `json:"queryParams,omitempty"`
	Body        []HeaderKeyValue `json:"body,omitempty"`
}

// ResponseTransform represents response transformation settings.
type ResponseTransform struct {
	Allow   *ResponseTransformAllow   `json:"allow,omitempty"`
	Remove  *ResponseTransformRemove  `json:"remove,omitempty"`
	Rename  *ResponseTransformRename  `json:"rename,omitempty"`
	Replace *ResponseTransformReplace `json:"replace,omitempty"`
	Add     *ResponseTransformAdd     `json:"add,omitempty"`
	Append  *ResponseTransformAppend  `json:"append,omitempty"`
}

type ResponseTransformAllow struct {
	JSONKeys []string `json:"jsonKeys,omitempty"`
}

type ResponseTransformRemove struct {
	IfStatusCode []int    `json:"ifStatusCode,omitempty"`
	HeaderKeys   []string `json:"headerKeys,omitempty"`
	JSONKeys     []string `json:"jsonKeys,omitempty"`
}

type ResponseTransformRename struct {
	IfStatusCode []int            `json:"ifStatusCode,omitempty"`
	Headers      []RenameKeyValue `json:"headers,omitempty"`
	JSON         []RenameKeyValue `json:"json,omitempty"`
}

type ResponseTransformReplace struct {
	IfStatusCode []int            `json:"ifStatusCode,omitempty"`
	Headers      []HeaderKeyValue `json:"headers,omitempty"`
	JSON         []HeaderKeyValue `json:"json,omitempty"`
	Body         string           `json:"body,omitempty"`
}

type ResponseTransformAdd struct {
	IfStatusCode []int            `json:"ifStatusCode,omitempty"`
	Headers      []HeaderKeyValue `json:"headers,omitempty"`
	JSON         []HeaderKeyValue `json:"json,omitempty"`
}

type ResponseTransformAppend struct {
	IfStatusCode []int            `json:"ifStatusCode,omitempty"`
	Headers      []HeaderKeyValue `json:"headers,omitempty"`
	JSON         []HeaderKeyValue `json:"json,omitempty"`
}

// User represents an API Gateway user.
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	CustomID string `json:"customID,omitempty"`
}

// CreateUserRequest is the request body for POST /users.
type CreateUserRequest struct {
	Name     string `json:"name"`
	CustomID string `json:"customID,omitempty"`
}

// UserAuthentication represents user authentication settings.
type UserAuthentication struct {
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`
	JWT       *JWTAuth   `json:"jwt,omitempty"`
	HMACAuth  *HMACAuth  `json:"hmacAuth,omitempty"`
}

type BasicAuth struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

type JWTAuth struct {
	Key       string `json:"key"`
	Secret    string `json:"secret"`
	Algorithm string `json:"algorithm"`
}

type HMACAuth struct {
	UserName string `json:"userName"`
	Secret   string `json:"secret"`
}

// Group represents an API Gateway group.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CreateGroupRequest is the request body for POST /groups.
type CreateGroupRequest struct {
	Name string `json:"name"`
}

// UserGroups represents user group assignments.
type UserGroups struct {
	Groups []GroupRef `json:"groups"`
}

type GroupRef struct {
	ID string `json:"id"`
}

// RouteAuthorization represents route authorization (ACL) settings.
type RouteAuthorization struct {
	IsACLEnabled bool            `json:"isACLEnabled"`
	Groups       []ACLGroupEntry `json:"groups,omitempty"`
}

type ACLGroupEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Enabled bool   `json:"enabled"`
}

// OIDCConfig represents an OIDC configuration.
type OIDCConfig struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	AuthenticationMethods []string `json:"authenticationMethods,omitempty"`
	Issuer                string   `json:"issuer"`
	ClientID              string   `json:"clientId"`
	ClientSecret          string   `json:"clientSecret,omitempty"`
	Scopes                []string `json:"scopes,omitempty"`
	HideCredentials       bool     `json:"hideCredentials,omitempty"`
	TokenAudiences        []string `json:"tokenAudiences,omitempty"`
	UseSession            bool     `json:"useSession,omitempty"`
}

// CreateOIDCRequest is the request body for POST /oidc.
type CreateOIDCRequest struct {
	Name                  string   `json:"name"`
	AuthenticationMethods []string `json:"authenticationMethods,omitempty"`
	Issuer                string   `json:"issuer"`
	ClientID              string   `json:"clientId"`
	ClientSecret          string   `json:"clientSecret"`
	Scopes                []string `json:"scopes,omitempty"`
	TokenAudiences        []string `json:"tokenAudiences,omitempty"`
}

// Domain represents a custom domain.
type Domain struct {
	ID            string `json:"id"`
	DomainName    string `json:"domainName"`
	CertificateID string `json:"certificateId,omitempty"`
}

// CreateDomainRequest is the request body for POST /domains.
type CreateDomainRequest struct {
	DomainName    string `json:"domainName"`
	CertificateID string `json:"certificateId,omitempty"`
}

// UpdateDomainRequest is the request body for PUT /domains/{id}.
type UpdateDomainRequest struct {
	CertificateID string `json:"certificateId,omitempty"`
}

// Certificate represents a TLS certificate.
type Certificate struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	ExpiredAt string   `json:"expiredAt,omitempty"`
}

// CreateCertificateRequest is the request body for POST /certificates.
type CreateCertificateRequest struct {
	Name  string   `json:"name"`
	RSA   *CertKeyPair `json:"rsa,omitempty"`
	ECDSA *CertKeyPair `json:"ecdsa,omitempty"`
}

type CertKeyPair struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

// UpdateCertificateRequest is the request body for PUT /certificates/{id}.
type UpdateCertificateRequest struct {
	Name  string   `json:"name,omitempty"`
	RSA   *CertKeyPair `json:"rsa,omitempty"`
	ECDSA *CertKeyPair `json:"ecdsa,omitempty"`
}

// ErrorResponse represents a Sakura API error.
type ErrorResponse struct {
	Message string `json:"message"`
}
