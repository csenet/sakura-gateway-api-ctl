package sakura

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultBaseURL = "https://secure.sakura.ad.jp/cloud/api/apigw/1.0"
)

// Client is the interface for the Sakura Cloud API Gateway API.
type Client interface {
	// Subscriptions
	ListPlans(ctx context.Context) ([]Plan, error)
	CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (*Subscription, error)
	GetSubscription(ctx context.Context, id string) (*Subscription, error)
	ListSubscriptions(ctx context.Context) ([]Subscription, error)
	UpdateSubscription(ctx context.Context, id string, req UpdateSubscriptionRequest) error
	DeleteSubscription(ctx context.Context, id string) error

	// Services
	CreateService(ctx context.Context, req CreateServiceRequest) (*Service, error)
	GetService(ctx context.Context, id string) (*Service, error)
	ListServices(ctx context.Context) ([]Service, error)
	UpdateService(ctx context.Context, id string, req UpdateServiceRequest) error
	DeleteService(ctx context.Context, id string) error

	// Routes
	CreateRoute(ctx context.Context, serviceID string, req CreateRouteRequest) (*Route, error)
	GetRoute(ctx context.Context, serviceID, routeID string) (*Route, error)
	ListRoutes(ctx context.Context, serviceID string) ([]Route, error)
	UpdateRoute(ctx context.Context, serviceID, routeID string, req UpdateRouteRequest) error
	DeleteRoute(ctx context.Context, serviceID, routeID string) error

	// Request/Response Transformations
	SetRequestTransform(ctx context.Context, serviceID, routeID string, req RequestTransform) error
	GetRequestTransform(ctx context.Context, serviceID, routeID string) (*RequestTransform, error)
	SetResponseTransform(ctx context.Context, serviceID, routeID string, req ResponseTransform) error
	GetResponseTransform(ctx context.Context, serviceID, routeID string) (*ResponseTransform, error)

	// Users
	CreateUser(ctx context.Context, req CreateUserRequest) (*User, error)
	GetUser(ctx context.Context, id string) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	DeleteUser(ctx context.Context, id string) error
	SetUserAuthentication(ctx context.Context, userID string, auth UserAuthentication) error
	SetUserGroups(ctx context.Context, userID string, groups UserGroups) error

	// Groups
	CreateGroup(ctx context.Context, req CreateGroupRequest) (*Group, error)
	ListGroups(ctx context.Context) ([]Group, error)
	DeleteGroup(ctx context.Context, id string) error

	// Route Authorization
	SetRouteAuthorization(ctx context.Context, serviceID, routeID string, auth RouteAuthorization) error
	GetRouteAuthorization(ctx context.Context, serviceID, routeID string) (*RouteAuthorization, error)

	// OIDC
	CreateOIDC(ctx context.Context, req CreateOIDCRequest) (*OIDCConfig, error)
	GetOIDC(ctx context.Context, id string) (*OIDCConfig, error)
	ListOIDC(ctx context.Context) ([]OIDCConfig, error)
	DeleteOIDC(ctx context.Context, id string) error

	// Domains
	CreateDomain(ctx context.Context, req CreateDomainRequest) (*Domain, error)
	ListDomains(ctx context.Context) ([]Domain, error)
	UpdateDomain(ctx context.Context, id string, req UpdateDomainRequest) error
	DeleteDomain(ctx context.Context, id string) error

	// Certificates
	CreateCertificate(ctx context.Context, req CreateCertificateRequest) (*Certificate, error)
	ListCertificates(ctx context.Context) ([]Certificate, error)
	UpdateCertificate(ctx context.Context, id string, req UpdateCertificateRequest) error
	DeleteCertificate(ctx context.Context, id string) error
}

// httpClient implements Client using the real Sakura Cloud API.
type httpClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
	secret     string
}

// NewClient creates a new Sakura Cloud API client.
func NewClient(token, secret string) Client {
	return &httpClient{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:  token,
		secret: secret,
	}
}

// APIError represents a Sakura API error with status code.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("sakura api error (status %d): %s", e.StatusCode, e.Message)
}

// IsNotFound returns true if the error is a 404 Not Found.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsConflict returns true if the error is a 409 Conflict.
func IsConflict(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusConflict
	}
	return false
}

func (c *httpClient) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.SetBasicAuth(c.token, c.secret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Message}
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	return resp, nil
}

func decodeResponse[T any](resp *http.Response) (*T, error) {
	defer resp.Body.Close()
	var wrapped apiResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &wrapped.APIGW, nil
}

func decodeListResponse[T any](resp *http.Response) ([]T, error) {
	defer resp.Body.Close()
	var wrapped apiListResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}
	return wrapped.APIGW, nil
}
