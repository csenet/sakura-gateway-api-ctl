package controller

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.sakura.io,resources=sakuragatewayconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// GatewayReconciler reconciles Gateway objects.
type GatewayReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	DryRun       bool
	SakuraClient sakura.Client
}

func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var gw gatewayv1.Gateway
	if err := r.Get(ctx, req.NamespacedName, &gw); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Resolve GatewayClass and verify it is ours
	gcName := string(gw.Spec.GatewayClassName)
	var gc gatewayv1.GatewayClass
	if err := r.Get(ctx, types.NamespacedName{Name: gcName}, &gc); err != nil {
		return ctrl.Result{}, err
	}
	if string(gc.Spec.ControllerName) != ControllerName {
		return ctrl.Result{}, nil
	}

	log.Info("reconciling Gateway", "namespace", gw.Namespace, "name", gw.Name)

	// Handle deletion
	if !gw.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &gw, &gc)
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(&gw, FinalizerName) {
		controllerutil.AddFinalizer(&gw, FinalizerName)
		if err := r.Update(ctx, &gw); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Resolve SakuraGatewayConfig
	config, sakuraClient, err := r.resolveConfig(ctx, &gc)
	if err != nil {
		r.setGatewayCondition(&gw, gatewayv1.GatewayConditionAccepted, metav1.ConditionFalse, "InvalidConfig", err.Error())
		r.Status().Update(ctx, &gw)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Ensure Sakura Service exists
	serviceID, routeHost, err := r.ensureSakuraService(ctx, sakuraClient, &gw, config)
	if err != nil {
		r.setGatewayCondition(&gw, gatewayv1.GatewayConditionAccepted, metav1.ConditionFalse, "ServiceCreationFailed", err.Error())
		r.Status().Update(ctx, &gw)
		return ctrl.Result{}, err
	}

	// Store service ID in annotation
	if gw.Annotations == nil {
		gw.Annotations = make(map[string]string)
	}
	gw.Annotations[AnnotationServiceID] = serviceID
	if err := r.Update(ctx, &gw); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure gateway verification secret
	if config.Spec.IsVerificationEnabled() {
		if err := r.ensureVerificationSecret(ctx, &gw, config); err != nil {
			log.Error(err, "failed to ensure verification secret")
		}
	}

	// Update status
	gw.Status.Addresses = []gatewayv1.GatewayStatusAddress{
		{
			Type:  ptrTo(gatewayv1.HostnameAddressType),
			Value: routeHost,
		},
	}
	r.setGatewayCondition(&gw, gatewayv1.GatewayConditionAccepted, metav1.ConditionTrue, "Accepted", "Gateway is accepted")
	r.setGatewayCondition(&gw, gatewayv1.GatewayConditionProgrammed, metav1.ConditionTrue, "Programmed", "Sakura API Gateway service is created")

	if err := r.Status().Update(ctx, &gw); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Gateway reconciled", "serviceID", serviceID, "routeHost", routeHost)
	return ctrl.Result{}, nil
}

func (r *GatewayReconciler) resolveConfig(ctx context.Context, gc *gatewayv1.GatewayClass) (*gwapiv1alpha1.SakuraGatewayConfig, sakura.Client, error) {
	if gc.Spec.ParametersRef == nil {
		return nil, nil, fmt.Errorf("GatewayClass %q has no parametersRef", gc.Name)
	}

	var config gwapiv1alpha1.SakuraGatewayConfig
	if err := r.Get(ctx, types.NamespacedName{Name: gc.Spec.ParametersRef.Name}, &config); err != nil {
		return nil, nil, fmt.Errorf("SakuraGatewayConfig %q not found: %w", gc.Spec.ParametersRef.Name, err)
	}

	if config.Status.SubscriptionID == "" {
		return nil, nil, fmt.Errorf("SakuraGatewayConfig %q has no subscriptionId", config.Name)
	}

	if r.DryRun && r.SakuraClient != nil {
		return &config, r.SakuraClient, nil
	}

	// Create client from credentials
	var secret corev1.Secret
	secretKey := types.NamespacedName{
		Namespace: config.Spec.CredentialsRef.Namespace,
		Name:      config.Spec.CredentialsRef.Name,
	}
	if err := r.Get(ctx, secretKey, &secret); err != nil {
		return nil, nil, fmt.Errorf("get credentials secret: %w", err)
	}

	token := string(secret.Data["access-token"])
	tokenSecret := string(secret.Data["access-token-secret"])
	return &config, sakura.NewClient(token, tokenSecret), nil
}

func (r *GatewayReconciler) ensureSakuraService(ctx context.Context, sakuraClient sakura.Client, gw *gatewayv1.Gateway, config *gwapiv1alpha1.SakuraGatewayConfig) (string, string, error) {
	// Check if service already exists
	if gw.Annotations != nil {
		if serviceID, ok := gw.Annotations[AnnotationServiceID]; ok && serviceID != "" {
			svc, err := sakuraClient.GetService(ctx, serviceID)
			if err == nil {
				return svc.ID, svc.RouteHost, nil
			}
			if !sakura.IsNotFound(err) {
				return "", "", err
			}
			// Service was deleted externally, recreate
		}
	}

	// Determine service name
	serviceName := fmt.Sprintf("%s_%s", gw.Namespace, gw.Name)
	if len(serviceName) > 255 {
		serviceName = serviceName[:255]
	}

	// Create service
	svc, err := sakuraClient.CreateService(ctx, sakura.CreateServiceRequest{
		Name:     serviceName,
		Protocol: "http",
		Host:     "placeholder.example.com", // Updated when HTTPRoute creates NodePort
		Subscription: &sakura.SubscriptionRef{
			ID: config.Status.SubscriptionID,
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("create sakura service: %w", err)
	}

	return svc.ID, svc.RouteHost, nil
}

func (r *GatewayReconciler) ensureVerificationSecret(ctx context.Context, gw *gatewayv1.Gateway, config *gwapiv1alpha1.SakuraGatewayConfig) error {
	secretName := fmt.Sprintf("%s-gw-secret", gw.Name)
	var secret corev1.Secret
	err := r.Get(ctx, types.NamespacedName{Namespace: gw.Namespace, Name: secretName}, &secret)
	if err == nil {
		// Secret already exists
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Generate random secret
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return fmt.Errorf("generate random secret: %w", err)
	}
	secretValue := base64.StdEncoding.EncodeToString(secretBytes)

	headerName := config.Spec.GetVerificationHeaderName()

	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: gw.Namespace,
			Labels: map[string]string{
				LabelManagedBy: LabelManagedByValue,
			},
		},
		Data: map[string][]byte{
			"header-name":  []byte(headerName),
			"header-value": []byte(secretValue),
		},
	}

	if err := ctrl.SetControllerReference(gw, &secret, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, &secret)
}

func (r *GatewayReconciler) reconcileDelete(ctx context.Context, gw *gatewayv1.Gateway, gc *gatewayv1.GatewayClass) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(gw, FinalizerName) {
		return ctrl.Result{}, nil
	}

	// Delete Sakura resources
	if serviceID, ok := gw.Annotations[AnnotationServiceID]; ok && serviceID != "" {
		config, sakuraClient, err := r.resolveConfig(ctx, gc)
		if err != nil {
			log.Error(err, "failed to resolve config during deletion, removing finalizer anyway")
		} else {
			_ = config // used for verification cleanup if needed

			// Delete all routes first
			routes, err := sakuraClient.ListRoutes(ctx, serviceID)
			if err == nil {
				for _, route := range routes {
					if delErr := sakuraClient.DeleteRoute(ctx, serviceID, route.ID); delErr != nil {
						log.Error(delErr, "failed to delete route", "routeID", route.ID)
					}
				}
			}

			// Delete service
			if err := sakuraClient.DeleteService(ctx, serviceID); err != nil && !sakura.IsNotFound(err) {
				log.Error(err, "failed to delete sakura service", "serviceID", serviceID)
			}
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(gw, FinalizerName)
	if err := r.Update(ctx, gw); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Gateway deleted", "name", gw.Name)
	return ctrl.Result{}, nil
}

func (r *GatewayReconciler) setGatewayCondition(gw *gatewayv1.Gateway, condType gatewayv1.GatewayConditionType, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition((*[]metav1.Condition)(&gw.Status.Conditions), metav1.Condition{
		Type:               string(condType),
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: gw.Generation,
	})
}

func (r *GatewayReconciler) findGatewaysForGatewayClass(ctx context.Context, obj client.Object) []reconcile.Request {
	gc, ok := obj.(*gatewayv1.GatewayClass)
	if !ok {
		return nil
	}

	var gatewayList gatewayv1.GatewayList
	if err := r.List(ctx, &gatewayList); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, gw := range gatewayList.Items {
		if string(gw.Spec.GatewayClassName) == gc.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: gw.Namespace,
					Name:      gw.Name,
				},
			})
		}
	}
	return requests
}

func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.Gateway{}).
		Owns(&corev1.Secret{}).
		Watches(&gatewayv1.GatewayClass{}, handler.EnqueueRequestsFromMapFunc(r.findGatewaysForGatewayClass)).
		Complete(r)
}

func ptrTo[T any](v T) *T {
	return &v
}
