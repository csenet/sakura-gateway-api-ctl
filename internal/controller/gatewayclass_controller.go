package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	gwapiv1alpha1 "github.com/sakura-cloud/sakura-gateway-api/api/v1alpha1"
)

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gatewayclasses/status,verbs=get;update;patch

// GatewayClassReconciler reconciles GatewayClass objects.
type GatewayClassReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *GatewayClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var gc gatewayv1.GatewayClass
	if err := r.Get(ctx, req.NamespacedName, &gc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Only process GatewayClasses for our controller
	if string(gc.Spec.ControllerName) != ControllerName {
		return ctrl.Result{}, nil
	}

	log.Info("reconciling GatewayClass", "name", gc.Name)

	// Validate parametersRef
	if gc.Spec.ParametersRef != nil {
		if err := r.validateParametersRef(ctx, &gc); err != nil {
			meta.SetStatusCondition((*[]metav1.Condition)(&gc.Status.Conditions), metav1.Condition{
				Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
				Status:             metav1.ConditionFalse,
				Reason:             string(gatewayv1.GatewayClassReasonInvalidParameters),
				Message:            err.Error(),
				ObservedGeneration: gc.Generation,
			})
			if updateErr := r.Status().Update(ctx, &gc); updateErr != nil {
				log.Error(updateErr, "failed to update status")
			}
			return ctrl.Result{}, nil
		}
	}

	// Mark as accepted
	meta.SetStatusCondition((*[]metav1.Condition)(&gc.Status.Conditions), metav1.Condition{
		Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
		Status:             metav1.ConditionTrue,
		Reason:             string(gatewayv1.GatewayClassReasonAccepted),
		Message:            "GatewayClass is accepted",
		ObservedGeneration: gc.Generation,
	})

	if err := r.Status().Update(ctx, &gc); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("GatewayClass accepted", "name", gc.Name)
	return ctrl.Result{}, nil
}

func (r *GatewayClassReconciler) validateParametersRef(ctx context.Context, gc *gatewayv1.GatewayClass) error {
	ref := gc.Spec.ParametersRef
	if string(ref.Group) != "gateway.sakura.io" || ref.Kind != "SakuraGatewayConfig" {
		return fmt.Errorf("parametersRef must reference gateway.sakura.io/SakuraGatewayConfig, got %s/%s", ref.Group, ref.Kind)
	}

	var config gwapiv1alpha1.SakuraGatewayConfig
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name}, &config); err != nil {
		return fmt.Errorf("SakuraGatewayConfig %q not found: %w", ref.Name, err)
	}

	// Check if the config is accepted
	for _, cond := range config.Status.Conditions {
		if cond.Type == "Accepted" && cond.Status == metav1.ConditionTrue {
			return nil
		}
	}

	return fmt.Errorf("SakuraGatewayConfig %q is not accepted yet", ref.Name)
}

func (r *GatewayClassReconciler) findGatewayClassesForConfig(ctx context.Context, obj client.Object) []reconcile.Request {
	config, ok := obj.(*gwapiv1alpha1.SakuraGatewayConfig)
	if !ok {
		return nil
	}

	var gcList gatewayv1.GatewayClassList
	if err := r.List(ctx, &gcList); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, gc := range gcList.Items {
		if gc.Spec.ParametersRef != nil && gc.Spec.ParametersRef.Name == config.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: gc.Name},
			})
		}
	}
	return requests
}

func (r *GatewayClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1.GatewayClass{}).
		Watches(&gwapiv1alpha1.SakuraGatewayConfig{}, handler.EnqueueRequestsFromMapFunc(r.findGatewayClassesForConfig)).
		Complete(r)
}
