package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	gwapiv1alpha1 "github.com/sakura-cloud/sakura-gateway-api/api/v1alpha1"
	"github.com/sakura-cloud/sakura-gateway-api/internal/sakura"
)

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// HTTPRouteReconciler reconciles HTTPRoute objects.
// Each backendRef creates a separate Sakura Service + Route.
type HTTPRouteReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	DryRun       bool
	SakuraClient sakura.Client
}

func (r *HTTPRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var hr gatewayv1.HTTPRoute
	if err := r.Get(ctx, req.NamespacedName, &hr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling HTTPRoute", "namespace", hr.Namespace, "name", hr.Name)

	// Handle deletion
	if !hr.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &hr)
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(&hr, FinalizerName) {
		controllerutil.AddFinalizer(&hr, FinalizerName)
		if err := r.Update(ctx, &hr); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Process each parentRef
	for _, parentRef := range hr.Spec.ParentRefs {
		if err := r.reconcileParent(ctx, &hr, parentRef); err != nil {
			log.Error(err, "failed to reconcile parent", "parent", parentRef.Name)
			r.setRouteCondition(&hr, parentRef, "Accepted", metav1.ConditionFalse, "ReconcileError", err.Error())
			r.Status().Update(ctx, &hr)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		r.setRouteCondition(&hr, parentRef, "Accepted", metav1.ConditionTrue, "Accepted", "Route is accepted")
		r.setRouteCondition(&hr, parentRef, "ResolvedRefs", metav1.ConditionTrue, "ResolvedRefs", "References resolved")
	}

	if err := r.Status().Update(ctx, &hr); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("HTTPRoute reconciled", "name", hr.Name)
	return ctrl.Result{}, nil
}

func (r *HTTPRouteReconciler) reconcileParent(ctx context.Context, hr *gatewayv1.HTTPRoute, parentRef gatewayv1.ParentReference) error {
	log := log.FromContext(ctx)

	// Resolve parent Gateway
	gwNamespace := hr.Namespace
	if parentRef.Namespace != nil {
		gwNamespace = string(*parentRef.Namespace)
	}

	var gw gatewayv1.Gateway
	if err := r.Get(ctx, types.NamespacedName{Namespace: gwNamespace, Name: string(parentRef.Name)}, &gw); err != nil {
		return fmt.Errorf("get parent gateway %s: %w", parentRef.Name, err)
	}

	// Get subscription ID from Gateway annotation
	subscriptionID := ""
	if gw.Annotations != nil {
		subscriptionID = gw.Annotations[AnnotationSubscriptionID]
	}
	if subscriptionID == "" {
		return fmt.Errorf("gateway %s has no subscription ID (not yet provisioned)", gw.Name)
	}

	// Get Sakura client
	var gc gatewayv1.GatewayClass
	if err := r.Get(ctx, types.NamespacedName{Name: string(gw.Spec.GatewayClassName)}, &gc); err != nil {
		return fmt.Errorf("get gatewayclass: %w", err)
	}
	sakuraClient, err := r.getSakuraClient(ctx, &gc)
	if err != nil {
		return err
	}

	// Get verification secret if available
	verHeaderName, verHeaderValue := r.getVerificationSecret(ctx, &gw)

	// Track Sakura service IDs and route IDs
	serviceIDs := r.getAnnotationMap(hr, AnnotationServiceIDs)
	routeIDs := r.getAnnotationMap(hr, AnnotationRouteIDs)
	npm := &NodePortManager{Client: r.Client, Scheme: r.Scheme}

	// Process each rule: each backendRef gets its own Sakura Service
	for i, rule := range hr.Spec.Rules {
		for _, backendRef := range rule.BackendRefs {
			if backendRef.Group != nil && *backendRef.Group != "" && *backendRef.Group != "core" {
				continue
			}
			if backendRef.Kind != nil && *backendRef.Kind != "Service" {
				continue
			}

			backendName := string(backendRef.Name)
			backendPort := int32(0)
			if backendRef.Port != nil {
				backendPort = int32(*backendRef.Port)
			}

			// 1. Ensure NodePort Service
			npResult, err := npm.EnsureNodePortService(ctx, hr, backendName, backendPort, hr.Namespace)
			if err != nil {
				return fmt.Errorf("ensure nodeport for %s: %w", backendName, err)
			}

			// 2. Ensure Sakura Service (one per backendRef)
			serviceKey := backendName
			serviceID, err := r.ensureSakuraService(ctx, sakuraClient, serviceIDs[serviceKey], hr, backendName, npResult, subscriptionID)
			if err != nil {
				return fmt.Errorf("ensure sakura service for %s: %w", backendName, err)
			}
			serviceIDs[serviceKey] = serviceID

			// 3. Create/Update Route for this rule on this Service
			ruleKey := fmt.Sprintf("%s-rule-%d", backendName, i)
			routeID, err := r.ensureSakuraRoute(ctx, sakuraClient, serviceID, hr, i, rule)
			if err != nil {
				return fmt.Errorf("ensure route for rule %d: %w", i, err)
			}
			routeIDs[ruleKey] = routeID

			// 4. Set request transform (shared secret + filters)
			if err := r.setRequestTransform(ctx, sakuraClient, serviceID, routeID, rule, verHeaderName, verHeaderValue); err != nil {
				log.Error(err, "failed to set request transform", "routeID", routeID)
			}

			// 5. Set response transform if needed
			if err := r.setResponseTransform(ctx, sakuraClient, serviceID, routeID, rule); err != nil {
				log.Error(err, "failed to set response transform", "routeID", routeID)
			}
		}
	}

	// Save annotations
	r.setAnnotationMap(hr, AnnotationServiceIDs, serviceIDs)
	r.setAnnotationMap(hr, AnnotationRouteIDs, routeIDs)
	if err := r.Update(ctx, hr); err != nil {
		return err
	}

	return nil
}

func (r *HTTPRouteReconciler) ensureSakuraService(ctx context.Context, sakuraClient sakura.Client, existingServiceID string, hr *gatewayv1.HTTPRoute, backendName string, npResult *NodePortResult, subscriptionID string) (string, error) {
	serviceName := fmt.Sprintf("%s_%s_%s", hr.Namespace, hr.Name, backendName)
	if len(serviceName) > 255 {
		serviceName = serviceName[:255]
	}

	port := int(npResult.NodePort)

	// Check if service already exists
	if existingServiceID != "" {
		svc, err := sakuraClient.GetService(ctx, existingServiceID)
		if err == nil {
			// Update host/port if changed
			if svc.Host != npResult.ExternalIP || (svc.Port != nil && *svc.Port != port) {
				if err := sakuraClient.UpdateService(ctx, existingServiceID, sakura.UpdateServiceRequest{
					Name:     svc.Name,
					Protocol: svc.Protocol,
					Host:     npResult.ExternalIP,
					Port:     &port,
				}); err != nil {
					return "", fmt.Errorf("update service host: %w", err)
				}
			}
			return existingServiceID, nil
		}
		if !sakura.IsNotFound(err) {
			return "", err
		}
		// Service deleted externally, recreate below
	}

	// Create new Sakura Service
	svc, err := sakuraClient.CreateService(ctx, sakura.CreateServiceRequest{
		Name:     serviceName,
		Protocol: "http",
		Host:     npResult.ExternalIP,
		Port:     &port,
		Subscription: &sakura.SubscriptionRef{
			ID: subscriptionID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("create sakura service: %w", err)
	}

	return svc.ID, nil
}

func (r *HTTPRouteReconciler) ensureSakuraRoute(ctx context.Context, sakuraClient sakura.Client, serviceID string, hr *gatewayv1.HTTPRoute, ruleIdx int, rule gatewayv1.HTTPRouteRule) (string, error) {
	routeName := fmt.Sprintf("%s_%s_rule-%d", hr.Namespace, hr.Name, ruleIdx)
	path := "/"
	var methods []string
	for _, match := range rule.Matches {
		if match.Path != nil && match.Path.Value != nil {
			path = *match.Path.Value
		}
		if match.Method != nil {
			methods = append(methods, string(*match.Method))
		}
	}
	if len(methods) == 0 {
		methods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	}

	var hosts []string
	for _, hostname := range hr.Spec.Hostnames {
		hosts = append(hosts, string(hostname))
	}

	// Always create new route (delete old one first if exists via annotation)
	route, err := sakuraClient.CreateRoute(ctx, serviceID, sakura.CreateRouteRequest{
		Name:      routeName,
		Protocols: "http,https",
		Path:      path,
		Methods:   methods,
		Hosts:     hosts,
	})
	if err != nil {
		// If conflict (same host+path), try to find and reuse existing route
		if sakura.IsConflict(err) {
			routes, listErr := sakuraClient.ListRoutes(ctx, serviceID)
			if listErr != nil {
				return "", err
			}
			for _, r := range routes {
				if r.Name == routeName {
					return r.ID, nil
				}
			}
		}
		return "", err
	}

	return route.ID, nil
}

func (r *HTTPRouteReconciler) setRequestTransform(ctx context.Context, sakuraClient sakura.Client, serviceID, routeID string, rule gatewayv1.HTTPRouteRule, verHeaderName, verHeaderValue string) error {
	transform := sakura.RequestTransform{}

	if verHeaderName != "" && verHeaderValue != "" {
		transform.Remove = &sakura.RequestTransformRemove{
			HeaderKeys: []string{verHeaderName},
		}
		transform.Add = &sakura.RequestTransformAdd{
			Headers: []sakura.HeaderKeyValue{
				{Key: verHeaderName, Value: verHeaderValue},
			},
		}
	}

	for _, filter := range rule.Filters {
		if filter.Type == gatewayv1.HTTPRouteFilterRequestHeaderModifier && filter.RequestHeaderModifier != nil {
			mod := filter.RequestHeaderModifier
			for _, h := range mod.Add {
				if transform.Add == nil {
					transform.Add = &sakura.RequestTransformAdd{}
				}
				transform.Add.Headers = append(transform.Add.Headers, sakura.HeaderKeyValue{
					Key: string(h.Name), Value: h.Value,
				})
			}
			for _, h := range mod.Set {
				if transform.Replace == nil {
					transform.Replace = &sakura.RequestTransformReplace{}
				}
				transform.Replace.Headers = append(transform.Replace.Headers, sakura.HeaderKeyValue{
					Key: string(h.Name), Value: h.Value,
				})
			}
			for _, name := range mod.Remove {
				if transform.Remove == nil {
					transform.Remove = &sakura.RequestTransformRemove{}
				}
				transform.Remove.HeaderKeys = append(transform.Remove.HeaderKeys, name)
			}
		}
	}

	return sakuraClient.SetRequestTransform(ctx, serviceID, routeID, transform)
}

func (r *HTTPRouteReconciler) setResponseTransform(ctx context.Context, sakuraClient sakura.Client, serviceID, routeID string, rule gatewayv1.HTTPRouteRule) error {
	transform := sakura.ResponseTransform{}
	hasTransform := false

	for _, filter := range rule.Filters {
		if filter.Type == gatewayv1.HTTPRouteFilterResponseHeaderModifier && filter.ResponseHeaderModifier != nil {
			hasTransform = true
			mod := filter.ResponseHeaderModifier
			for _, h := range mod.Add {
				if transform.Add == nil {
					transform.Add = &sakura.ResponseTransformAdd{}
				}
				transform.Add.Headers = append(transform.Add.Headers, sakura.HeaderKeyValue{
					Key: string(h.Name), Value: h.Value,
				})
			}
			for _, h := range mod.Set {
				if transform.Replace == nil {
					transform.Replace = &sakura.ResponseTransformReplace{}
				}
				transform.Replace.Headers = append(transform.Replace.Headers, sakura.HeaderKeyValue{
					Key: string(h.Name), Value: h.Value,
				})
			}
			for _, name := range mod.Remove {
				if transform.Remove == nil {
					transform.Remove = &sakura.ResponseTransformRemove{}
				}
				transform.Remove.HeaderKeys = append(transform.Remove.HeaderKeys, name)
			}
		}
	}

	if !hasTransform {
		return nil
	}

	return sakuraClient.SetResponseTransform(ctx, serviceID, routeID, transform)
}

func (r *HTTPRouteReconciler) reconcileDelete(ctx context.Context, hr *gatewayv1.HTTPRoute) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(hr, FinalizerName) {
		return ctrl.Result{}, nil
	}

	// Get Sakura client
	sakuraClient, err := r.getSakuraClientForDeletion(ctx, hr)

	// Delete Sakura routes and services
	if err == nil {
		routeIDs := r.getAnnotationMap(hr, AnnotationRouteIDs)
		serviceIDs := r.getAnnotationMap(hr, AnnotationServiceIDs)

		// Delete routes first
		for _, serviceID := range serviceIDs {
			for ruleKey, routeID := range routeIDs {
				if err := sakuraClient.DeleteRoute(ctx, serviceID, routeID); err != nil && !sakura.IsNotFound(err) {
					log.Error(err, "failed to delete route", "ruleKey", ruleKey, "routeID", routeID)
				}
			}
		}

		// Delete services
		for backendName, serviceID := range serviceIDs {
			if err := sakuraClient.DeleteService(ctx, serviceID); err != nil && !sakura.IsNotFound(err) {
				log.Error(err, "failed to delete sakura service", "backend", backendName, "serviceID", serviceID)
			}
		}
	} else {
		log.Error(err, "failed to get sakura client for cleanup, removing finalizer anyway")
	}

	// Delete managed NodePort services
	npm := &NodePortManager{Client: r.Client, Scheme: r.Scheme}
	for _, rule := range hr.Spec.Rules {
		for _, backendRef := range rule.BackendRefs {
			if err := npm.DeleteNodePortService(ctx, string(backendRef.Name), hr.Namespace); err != nil {
				log.Error(err, "failed to delete nodeport service")
			}
		}
	}

	controllerutil.RemoveFinalizer(hr, FinalizerName)
	if err := r.Update(ctx, hr); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("HTTPRoute deleted", "name", hr.Name)
	return ctrl.Result{}, nil
}

func (r *HTTPRouteReconciler) getVerificationSecret(ctx context.Context, gw *gatewayv1.Gateway) (string, string) {
	secretName := fmt.Sprintf("%s-gw-secret", gw.Name)
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: gw.Namespace, Name: secretName}, &secret); err != nil {
		return "", ""
	}
	return string(secret.Data["header-name"]), string(secret.Data["header-value"])
}

func (r *HTTPRouteReconciler) getSakuraClient(ctx context.Context, gc *gatewayv1.GatewayClass) (sakura.Client, error) {
	if r.DryRun && r.SakuraClient != nil {
		return r.SakuraClient, nil
	}

	if gc.Spec.ParametersRef == nil {
		return nil, fmt.Errorf("GatewayClass %q has no parametersRef", gc.Name)
	}

	var config gwapiv1alpha1.SakuraGatewayConfig
	if err := r.Get(ctx, types.NamespacedName{Name: gc.Spec.ParametersRef.Name}, &config); err != nil {
		return nil, fmt.Errorf("get SakuraGatewayConfig %q: %w", gc.Spec.ParametersRef.Name, err)
	}

	var secret corev1.Secret
	secretKey := types.NamespacedName{
		Namespace: config.Spec.CredentialsRef.Namespace,
		Name:      config.Spec.CredentialsRef.Name,
	}
	if err := r.Get(ctx, secretKey, &secret); err != nil {
		return nil, fmt.Errorf("get credentials secret: %w", err)
	}

	return sakura.NewClient(string(secret.Data["access-token"]), string(secret.Data["access-token-secret"])), nil
}

func (r *HTTPRouteReconciler) getSakuraClientForDeletion(ctx context.Context, hr *gatewayv1.HTTPRoute) (sakura.Client, error) {
	for _, parentRef := range hr.Spec.ParentRefs {
		gwNamespace := hr.Namespace
		if parentRef.Namespace != nil {
			gwNamespace = string(*parentRef.Namespace)
		}
		var gw gatewayv1.Gateway
		if err := r.Get(ctx, types.NamespacedName{Namespace: gwNamespace, Name: string(parentRef.Name)}, &gw); err != nil {
			continue
		}
		var gc gatewayv1.GatewayClass
		if err := r.Get(ctx, types.NamespacedName{Name: string(gw.Spec.GatewayClassName)}, &gc); err != nil {
			continue
		}
		client, err := r.getSakuraClient(ctx, &gc)
		if err == nil {
			return client, nil
		}
	}
	return nil, fmt.Errorf("could not resolve sakura client from any parentRef")
}

func (r *HTTPRouteReconciler) getAnnotationMap(hr *gatewayv1.HTTPRoute, key string) map[string]string {
	result := make(map[string]string)
	if hr.Annotations != nil {
		if data, ok := hr.Annotations[key]; ok {
			json.Unmarshal([]byte(data), &result)
		}
	}
	return result
}

func (r *HTTPRouteReconciler) setAnnotationMap(hr *gatewayv1.HTTPRoute, key string, m map[string]string) {
	if hr.Annotations == nil {
		hr.Annotations = make(map[string]string)
	}
	data, _ := json.Marshal(m)
	hr.Annotations[key] = string(data)
}

func (r *HTTPRouteReconciler) setRouteCondition(hr *gatewayv1.HTTPRoute, parentRef gatewayv1.ParentReference, condType string, status metav1.ConditionStatus, reason, message string) {
	var parentStatus *gatewayv1.RouteParentStatus
	for i := range hr.Status.Parents {
		if hr.Status.Parents[i].ParentRef.Name == parentRef.Name {
			parentStatus = &hr.Status.Parents[i]
			break
		}
	}
	if parentStatus == nil {
		hr.Status.Parents = append(hr.Status.Parents, gatewayv1.RouteParentStatus{
			ParentRef:      parentRef,
			ControllerName: gatewayv1.GatewayController(ControllerName),
		})
		parentStatus = &hr.Status.Parents[len(hr.Status.Parents)-1]
	}

	meta.SetStatusCondition((*[]metav1.Condition)(&parentStatus.Conditions), metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: hr.Generation,
	})
}

func (r *HTTPRouteReconciler) findHTTPRoutesForGateway(ctx context.Context, obj client.Object) []reconcile.Request {
	gw, ok := obj.(*gatewayv1.Gateway)
	if !ok {
		return nil
	}

	var routeList gatewayv1.HTTPRouteList
	if err := r.List(ctx, &routeList, client.InNamespace(gw.Namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, route := range routeList.Items {
		for _, parentRef := range route.Spec.ParentRefs {
			if string(parentRef.Name) == gw.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: route.Namespace,
						Name:      route.Name,
					},
				})
				break
			}
		}
	}
	return requests
}

func (r *HTTPRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.HTTPRoute{}).
		Owns(&corev1.Service{}).
		Watches(&gatewayv1.Gateway{}, handler.EnqueueRequestsFromMapFunc(r.findHTTPRoutesForGateway)).
		Complete(r)
}
