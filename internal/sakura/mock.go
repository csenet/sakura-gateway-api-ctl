package sakura

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// MockClient implements Client with in-memory state for dry-run testing.
type MockClient struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription
	services      map[string]*Service
	routes        map[string]map[string]*Route // serviceID -> routeID -> Route
	reqTransforms map[string]*RequestTransform  // serviceID/routeID -> transform
	resTransforms map[string]*ResponseTransform
	users         map[string]*User
	groups        map[string]*Group
	oidcs         map[string]*OIDCConfig
	domains       map[string]*Domain
	certificates  map[string]*Certificate
	routeAuths    map[string]*RouteAuthorization // serviceID/routeID -> auth
}

// NewMockClient creates a new mock client for dry-run testing.
func NewMockClient() Client {
	return &MockClient{
		subscriptions: make(map[string]*Subscription),
		services:      make(map[string]*Service),
		routes:        make(map[string]map[string]*Route),
		reqTransforms: make(map[string]*RequestTransform),
		resTransforms: make(map[string]*ResponseTransform),
		users:         make(map[string]*User),
		groups:        make(map[string]*Group),
		oidcs:         make(map[string]*OIDCConfig),
		domains:       make(map[string]*Domain),
		certificates:  make(map[string]*Certificate),
		routeAuths:    make(map[string]*RouteAuthorization),
	}
}

func newID() string {
	return uuid.New().String()
}

func transformKey(serviceID, routeID string) string {
	return serviceID + "/" + routeID
}

// Subscriptions

func (m *MockClient) ListPlans(_ context.Context) ([]Plan, error) {
	return []Plan{
		{ID: "plan-trial", Name: "Trial", Price: 0, MaxServices: 10, MaxRequests: 10000},
	}, nil
}

func (m *MockClient) CreateSubscription(_ context.Context, req CreateSubscriptionRequest) (*Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub := &Subscription{
		ID:   newID(),
		Name: req.Name,
		Plan: &Plan{ID: req.PlanID, Name: "Trial"},
	}
	m.subscriptions[sub.ID] = sub
	return sub, nil
}

func (m *MockClient) GetSubscription(_ context.Context, id string) (*Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sub, ok := m.subscriptions[id]
	if !ok {
		return nil, &APIError{StatusCode: 404, Message: "subscription not found"}
	}
	return sub, nil
}

func (m *MockClient) ListSubscriptions(_ context.Context) ([]Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Subscription
	for _, s := range m.subscriptions {
		result = append(result, *s)
	}
	return result, nil
}

func (m *MockClient) UpdateSubscription(_ context.Context, id string, req UpdateSubscriptionRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subscriptions[id]
	if !ok {
		return &APIError{StatusCode: 404, Message: "subscription not found"}
	}
	sub.Name = req.Name
	return nil
}

func (m *MockClient) DeleteSubscription(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subscriptions, id)
	return nil
}

// Services

func (m *MockClient) CreateService(_ context.Context, req CreateServiceRequest) (*Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	svc := &Service{
		ID:             newID(),
		Name:           req.Name,
		Protocol:       req.Protocol,
		Host:           req.Host,
		Path:           req.Path,
		Port:           req.Port,
		Authentication: req.Authentication,
		CorsConfig:     req.CorsConfig,
		RouteHost:      fmt.Sprintf("site-%s.mock.apigw.sakura.ne.jp", newID()[:8]),
		Subscription:   req.Subscription,
	}
	m.services[svc.ID] = svc
	m.routes[svc.ID] = make(map[string]*Route)
	return svc, nil
}

func (m *MockClient) GetService(_ context.Context, id string) (*Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	svc, ok := m.services[id]
	if !ok {
		return nil, &APIError{StatusCode: 404, Message: "service not found"}
	}
	return svc, nil
}

func (m *MockClient) ListServices(_ context.Context) ([]Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Service
	for _, s := range m.services {
		result = append(result, *s)
	}
	return result, nil
}

func (m *MockClient) UpdateService(_ context.Context, id string, req UpdateServiceRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	svc, ok := m.services[id]
	if !ok {
		return &APIError{StatusCode: 404, Message: "service not found"}
	}
	if req.Host != "" {
		svc.Host = req.Host
	}
	if req.Protocol != "" {
		svc.Protocol = req.Protocol
	}
	if req.Port != nil {
		svc.Port = req.Port
	}
	if req.Authentication != "" {
		svc.Authentication = req.Authentication
	}
	if req.CorsConfig != nil {
		svc.CorsConfig = req.CorsConfig
	}
	if req.OIDC != nil {
		svc.OIDC = req.OIDC
	}
	return nil
}

func (m *MockClient) DeleteService(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.services, id)
	delete(m.routes, id)
	return nil
}

// Routes

func (m *MockClient) CreateRoute(_ context.Context, serviceID string, req CreateRouteRequest) (*Route, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.services[serviceID]; !ok {
		return nil, &APIError{StatusCode: 404, Message: "service not found"}
	}
	route := &Route{
		ID:        newID(),
		ServiceID: serviceID,
		Name:      req.Name,
		Protocols: req.Protocols,
		Path:      req.Path,
		Hosts:     req.Hosts,
		Methods:   req.Methods,
	}
	if m.routes[serviceID] == nil {
		m.routes[serviceID] = make(map[string]*Route)
	}
	m.routes[serviceID][route.ID] = route
	return route, nil
}

func (m *MockClient) GetRoute(_ context.Context, serviceID, routeID string) (*Route, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	routes, ok := m.routes[serviceID]
	if !ok {
		return nil, &APIError{StatusCode: 404, Message: "service not found"}
	}
	route, ok := routes[routeID]
	if !ok {
		return nil, &APIError{StatusCode: 404, Message: "route not found"}
	}
	return route, nil
}

func (m *MockClient) ListRoutes(_ context.Context, serviceID string) ([]Route, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Route
	for _, r := range m.routes[serviceID] {
		result = append(result, *r)
	}
	return result, nil
}

func (m *MockClient) UpdateRoute(_ context.Context, serviceID, routeID string, req UpdateRouteRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	routes, ok := m.routes[serviceID]
	if !ok {
		return &APIError{StatusCode: 404, Message: "service not found"}
	}
	route, ok := routes[routeID]
	if !ok {
		return &APIError{StatusCode: 404, Message: "route not found"}
	}
	if req.Path != "" {
		route.Path = req.Path
	}
	if req.Methods != nil {
		route.Methods = req.Methods
	}
	if req.Hosts != nil {
		route.Hosts = req.Hosts
	}
	return nil
}

func (m *MockClient) DeleteRoute(_ context.Context, serviceID, routeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if routes, ok := m.routes[serviceID]; ok {
		delete(routes, routeID)
	}
	return nil
}

// Request/Response Transformations

func (m *MockClient) SetRequestTransform(_ context.Context, serviceID, routeID string, req RequestTransform) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqTransforms[transformKey(serviceID, routeID)] = &req
	return nil
}

func (m *MockClient) GetRequestTransform(_ context.Context, serviceID, routeID string) (*RequestTransform, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.reqTransforms[transformKey(serviceID, routeID)]
	if !ok {
		return &RequestTransform{}, nil
	}
	return t, nil
}

func (m *MockClient) SetResponseTransform(_ context.Context, serviceID, routeID string, req ResponseTransform) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resTransforms[transformKey(serviceID, routeID)] = &req
	return nil
}

func (m *MockClient) GetResponseTransform(_ context.Context, serviceID, routeID string) (*ResponseTransform, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.resTransforms[transformKey(serviceID, routeID)]
	if !ok {
		return &ResponseTransform{}, nil
	}
	return t, nil
}

// Users

func (m *MockClient) CreateUser(_ context.Context, req CreateUserRequest) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user := &User{ID: newID(), Name: req.Name, CustomID: req.CustomID}
	m.users[user.ID] = user
	return user, nil
}

func (m *MockClient) GetUser(_ context.Context, id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.users[id]
	if !ok {
		return nil, &APIError{StatusCode: 404, Message: "user not found"}
	}
	return user, nil
}

func (m *MockClient) ListUsers(_ context.Context) ([]User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []User
	for _, u := range m.users {
		result = append(result, *u)
	}
	return result, nil
}

func (m *MockClient) DeleteUser(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.users, id)
	return nil
}

func (m *MockClient) SetUserAuthentication(_ context.Context, _ string, _ UserAuthentication) error {
	return nil
}

func (m *MockClient) SetUserGroups(_ context.Context, _ string, _ UserGroups) error {
	return nil
}

// Groups

func (m *MockClient) CreateGroup(_ context.Context, req CreateGroupRequest) (*Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	group := &Group{ID: newID(), Name: req.Name}
	m.groups[group.ID] = group
	return group, nil
}

func (m *MockClient) ListGroups(_ context.Context) ([]Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Group
	for _, g := range m.groups {
		result = append(result, *g)
	}
	return result, nil
}

func (m *MockClient) DeleteGroup(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.groups, id)
	return nil
}

// Route Authorization

func (m *MockClient) SetRouteAuthorization(_ context.Context, serviceID, routeID string, auth RouteAuthorization) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routeAuths[transformKey(serviceID, routeID)] = &auth
	return nil
}

func (m *MockClient) GetRouteAuthorization(_ context.Context, serviceID, routeID string) (*RouteAuthorization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	auth, ok := m.routeAuths[transformKey(serviceID, routeID)]
	if !ok {
		return &RouteAuthorization{}, nil
	}
	return auth, nil
}

// OIDC

func (m *MockClient) CreateOIDC(_ context.Context, req CreateOIDCRequest) (*OIDCConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	oidc := &OIDCConfig{
		ID:       newID(),
		Name:     req.Name,
		Issuer:   req.Issuer,
		ClientID: req.ClientID,
	}
	m.oidcs[oidc.ID] = oidc
	return oidc, nil
}

func (m *MockClient) GetOIDC(_ context.Context, id string) (*OIDCConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	oidc, ok := m.oidcs[id]
	if !ok {
		return nil, &APIError{StatusCode: 404, Message: "oidc not found"}
	}
	return oidc, nil
}

func (m *MockClient) ListOIDC(_ context.Context) ([]OIDCConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []OIDCConfig
	for _, o := range m.oidcs {
		result = append(result, *o)
	}
	return result, nil
}

func (m *MockClient) DeleteOIDC(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.oidcs, id)
	return nil
}

// Domains

func (m *MockClient) CreateDomain(_ context.Context, req CreateDomainRequest) (*Domain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	domain := &Domain{
		ID:            newID(),
		DomainName:    req.DomainName,
		CertificateID: req.CertificateID,
	}
	m.domains[domain.ID] = domain
	return domain, nil
}

func (m *MockClient) ListDomains(_ context.Context) ([]Domain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Domain
	for _, d := range m.domains {
		result = append(result, *d)
	}
	return result, nil
}

func (m *MockClient) UpdateDomain(_ context.Context, id string, req UpdateDomainRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	domain, ok := m.domains[id]
	if !ok {
		return &APIError{StatusCode: 404, Message: "domain not found"}
	}
	domain.CertificateID = req.CertificateID
	return nil
}

func (m *MockClient) DeleteDomain(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.domains, id)
	return nil
}

// Certificates

func (m *MockClient) CreateCertificate(_ context.Context, req CreateCertificateRequest) (*Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cert := &Certificate{
		ID:   newID(),
		Name: req.Name,
	}
	m.certificates[cert.ID] = cert
	return cert, nil
}

func (m *MockClient) ListCertificates(_ context.Context) ([]Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Certificate
	for _, c := range m.certificates {
		result = append(result, *c)
	}
	return result, nil
}

func (m *MockClient) UpdateCertificate(_ context.Context, id string, _ UpdateCertificateRequest) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.certificates[id]; !ok {
		return &APIError{StatusCode: 404, Message: "certificate not found"}
	}
	return nil
}

func (m *MockClient) DeleteCertificate(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.certificates, id)
	return nil
}
