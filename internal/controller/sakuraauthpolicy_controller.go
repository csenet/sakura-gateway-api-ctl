package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gwapiv1alpha1 "github.com/sakura-cloud/sakura-gateway-api/api/v1alpha1"
)

// +kubebuilder:rbac:groups=gateway.sakura.io,resources=sakuraauthpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.sakura.io,resources=sakuraauthpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.sakura.io,resources=sakuraauthpolicies/finalizers,verbs=update

// SakuraAuthPolicyReconciler reconciles a SakuraAuthPolicy object.
// This is a stub implementation for Phase 1.
type SakuraAuthPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *SakuraAuthPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var policy gwapiv1alpha1.SakuraAuthPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling SakuraAuthPolicy (stub)", "namespace", policy.Namespace, "name", policy.Name)

	// Stub: just mark as accepted
	meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:               "Accepted",
		Status:             metav1.ConditionTrue,
		Reason:             "Accepted",
		Message:            "Policy accepted (auth enforcement not yet implemented)",
		ObservedGeneration: policy.Generation,
	})

	if err := r.Status().Update(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SakuraAuthPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gwapiv1alpha1.SakuraAuthPolicy{}).
		Complete(r)
}
