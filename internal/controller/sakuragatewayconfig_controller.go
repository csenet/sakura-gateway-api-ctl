package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gwapiv1alpha1 "github.com/sakura-cloud/sakura-gateway-api/api/v1alpha1"
	"github.com/sakura-cloud/sakura-gateway-api/internal/sakura"
)

// +kubebuilder:rbac:groups=gateway.sakura.io,resources=sakuragatewayconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.sakura.io,resources=sakuragatewayconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.sakura.io,resources=sakuragatewayconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// SakuraGatewayConfigReconciler reconciles a SakuraGatewayConfig object.
type SakuraGatewayConfigReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	DryRun       bool
	SakuraClient sakura.Client
}

func (r *SakuraGatewayConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var config gwapiv1alpha1.SakuraGatewayConfig
	if err := r.Get(ctx, req.NamespacedName, &config); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Early return if already reconciled for this generation
	if config.Status.SubscriptionID != "" && isConditionTrue(config.Status.Conditions, "Accepted", config.Generation) {
		return ctrl.Result{}, nil
	}

	log.Info("reconciling SakuraGatewayConfig", "name", config.Name)

	// Resolve credentials
	sakuraClient, err := r.getSakuraClient(ctx, &config)
	if err != nil {
		r.setStatus(&config, "", metav1.ConditionFalse, "InvalidCredentials",
			fmt.Sprintf("Failed to resolve credentials: %v", err))
		return ctrl.Result{}, err
	}

	// Resolve subscription
	subscriptionID, err := r.resolveSubscription(ctx, sakuraClient, &config)
	if err != nil {
		r.setStatus(&config, "", metav1.ConditionFalse, "SubscriptionError",
			fmt.Sprintf("Failed to resolve subscription: %v", err))
		return ctrl.Result{}, err
	}

	// Update status only if changed
	oldSubID := config.Status.SubscriptionID
	r.setStatus(&config, subscriptionID, metav1.ConditionTrue, "Valid", "Configuration is valid")

	if oldSubID != subscriptionID || !isConditionTrue(config.Status.Conditions, "Accepted", config.Generation) {
		if err := r.Status().Update(ctx, &config); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("SakuraGatewayConfig reconciled", "subscriptionID", subscriptionID)
	}

	return ctrl.Result{}, nil
}

func (r *SakuraGatewayConfigReconciler) setStatus(config *gwapiv1alpha1.SakuraGatewayConfig, subID string, status metav1.ConditionStatus, reason, message string) {
	if subID != "" {
		config.Status.SubscriptionID = subID
	}
	meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
		Type:               "Accepted",
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: config.Generation,
	})
	if status != metav1.ConditionTrue {
		if updateErr := r.Status().Update(context.Background(), config); updateErr != nil {
			// Best effort status update on error path
		}
	}
}

func isConditionTrue(conditions []metav1.Condition, condType string, generation int64) bool {
	for _, cond := range conditions {
		if cond.Type == condType && cond.Status == metav1.ConditionTrue && cond.ObservedGeneration >= generation {
			return true
		}
	}
	return false
}

func (r *SakuraGatewayConfigReconciler) getSakuraClient(ctx context.Context, config *gwapiv1alpha1.SakuraGatewayConfig) (sakura.Client, error) {
	if r.DryRun && r.SakuraClient != nil {
		return r.SakuraClient, nil
	}

	var secret corev1.Secret
	secretKey := types.NamespacedName{
		Namespace: config.Spec.CredentialsRef.Namespace,
		Name:      config.Spec.CredentialsRef.Name,
	}
	if err := r.Get(ctx, secretKey, &secret); err != nil {
		return nil, fmt.Errorf("get credentials secret %s/%s: %w", secretKey.Namespace, secretKey.Name, err)
	}

	token, ok := secret.Data["access-token"]
	if !ok {
		return nil, fmt.Errorf("credentials secret missing 'access-token' key")
	}
	tokenSecret, ok := secret.Data["access-token-secret"]
	if !ok {
		return nil, fmt.Errorf("credentials secret missing 'access-token-secret' key")
	}

	return sakura.NewClient(string(token), string(tokenSecret)), nil
}

func (r *SakuraGatewayConfigReconciler) resolveSubscription(ctx context.Context, sakuraClient sakura.Client, config *gwapiv1alpha1.SakuraGatewayConfig) (string, error) {
	// If existing ID is specified in spec, verify it
	if config.Spec.Subscription.ID != nil && *config.Spec.Subscription.ID != "" {
		sub, err := sakuraClient.GetSubscription(ctx, *config.Spec.Subscription.ID)
		if err != nil {
			return "", fmt.Errorf("get subscription %s: %w", *config.Spec.Subscription.ID, err)
		}
		if sub.Plan != nil {
			config.Status.PlanName = sub.Plan.Name
		}
		config.Status.MonthlyRequests = sub.MonthlyRequest
		return sub.ID, nil
	}

	// If we already created a subscription (stored in status), reuse it
	if config.Status.SubscriptionID != "" {
		sub, err := sakuraClient.GetSubscription(ctx, config.Status.SubscriptionID)
		if err == nil {
			if sub.Plan != nil {
				config.Status.PlanName = sub.Plan.Name
			}
			config.Status.MonthlyRequests = sub.MonthlyRequest
			return sub.ID, nil
		}
		// Subscription was deleted externally, recreate below
	}

	// Create a new subscription
	if config.Spec.Subscription.PlanID == nil || config.Spec.Subscription.Name == nil {
		return "", fmt.Errorf("subscription requires either 'id' or both 'planId' and 'name'")
	}

	sub, err := sakuraClient.CreateSubscription(ctx, sakura.CreateSubscriptionRequest{
		PlanID: *config.Spec.Subscription.PlanID,
		Name:   *config.Spec.Subscription.Name,
	})
	if err != nil {
		return "", fmt.Errorf("create subscription: %w", err)
	}

	return sub.ID, nil
}

func (r *SakuraGatewayConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gwapiv1alpha1.SakuraGatewayConfig{}).
		Complete(r)
}
