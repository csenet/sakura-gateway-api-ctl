package controller

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

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

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.sakura.io,resources=sakuragatewayconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// GatewayReconciler reconciles Gateway objects.
// Gateway = Sakura Subscription. Service creation is handled by HTTPRoute.
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
		return r.reconcileDelete(ctx, &gw)
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(&gw, FinalizerName) {
		controllerutil.AddFinalizer(&gw, FinalizerName)
		if err := r.Update(ctx, &gw); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Resolve SakuraGatewayConfig and verify subscription
	config, subscriptionID, err := r.resolveSubscription(ctx, &gc)
	if err != nil {
		r.setGatewayCondition(&gw, gatewayv1.GatewayConditionAccepted, metav1.ConditionFalse, "InvalidConfig", err.Error())
		r.Status().Update(ctx, &gw)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Store subscription ID in annotation for HTTPRoute to use
	if gw.Annotations == nil {
		gw.Annotations = make(map[string]string)
	}
	gw.Annotations[AnnotationSubscriptionID] = subscriptionID
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
	r.setGatewayCondition(&gw, gatewayv1.GatewayConditionAccepted, metav1.ConditionTrue, "Accepted", "Gateway is accepted")
	r.setGatewayCondition(&gw, gatewayv1.GatewayConditionProgrammed, metav1.ConditionTrue, "Programmed",
		fmt.Sprintf("Subscription %s is active", subscriptionID))

	if err := r.Status().Update(ctx, &gw); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Gateway reconciled", "subscriptionID", subscriptionID)
	return ctrl.Result{}, nil
}

func (r *GatewayReconciler) resolveSubscription(ctx context.Context, gc *gatewayv1.GatewayClass) (*gwapiv1alpha1.SakuraGatewayConfig, string, error) {
	if gc.Spec.ParametersRef == nil {
		return nil, "", fmt.Errorf("GatewayClass %q has no parametersRef", gc.Name)
	}

	var config gwapiv1alpha1.SakuraGatewayConfig
	if err := r.Get(ctx, types.NamespacedName{Name: gc.Spec.ParametersRef.Name}, &config); err != nil {
		return nil, "", fmt.Errorf("SakuraGatewayConfig %q not found: %w", gc.Spec.ParametersRef.Name, err)
	}

	if config.Status.SubscriptionID == "" {
		return nil, "", fmt.Errorf("SakuraGatewayConfig %q has no subscriptionId", config.Name)
	}

	return &config, config.Status.SubscriptionID, nil
}

func (r *GatewayReconciler) ensureVerificationSecret(ctx context.Context, gw *gatewayv1.Gateway, config *gwapiv1alpha1.SakuraGatewayConfig) error {
	secretName := fmt.Sprintf("%s-gw-secret", gw.Name)
	var secret corev1.Secret
	err := r.Get(ctx, types.NamespacedName{Namespace: gw.Namespace, Name: secretName}, &secret)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

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

func (r *GatewayReconciler) reconcileDelete(ctx context.Context, gw *gatewayv1.Gateway) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(gw, FinalizerName) {
		return ctrl.Result{}, nil
	}

	// Gateway = Subscription reference only; actual Sakura resources (Service/Route)
	// are cleaned up by the HTTPRoute controller.
	// Just remove the finalizer.

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
