package controller

import (
	"github.com/sakura-cloud/sakura-gateway-api/internal/sakura"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// ControllerName is the identifier for this controller in GatewayClass.spec.controllerName.
	ControllerName = "gateway.sakura.io/controller"

	// FinalizerName is the finalizer added to managed resources.
	FinalizerName = "gateway.sakura.io/finalizer"

	// Annotation keys for tracking Sakura resource IDs.
	AnnotationSubscriptionID = "gateway.sakura.io/subscription-id"
	AnnotationServiceIDs     = "gateway.sakura.io/service-ids"  // JSON map: backendName -> serviceID
	AnnotationRouteIDs       = "gateway.sakura.io/route-ids"    // JSON map: ruleKey -> routeID
	AnnotationDomainIDs      = "gateway.sakura.io/domain-ids"
	AnnotationCertificateIDs = "gateway.sakura.io/certificate-ids"

	// Label for managed resources.
	LabelManagedBy = "gateway.sakura.io/managed-by"
	LabelManagedByValue = "sakura-gateway-controller"
)

// SetupControllers registers all reconcilers with the manager.
func SetupControllers(mgr ctrl.Manager, dryRun bool) error {
	var sakuraClient sakura.Client
	if dryRun {
		sakuraClient = sakura.NewMockClient()
		ctrl.Log.WithName("setup").Info("using mock Sakura API client (dry-run mode)")
	}

	if err := (&SakuraGatewayConfigReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		DryRun:       dryRun,
		SakuraClient: sakuraClient,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&GatewayClassReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&GatewayReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		DryRun:       dryRun,
		SakuraClient: sakuraClient,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&HTTPRouteReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		DryRun:       dryRun,
		SakuraClient: sakuraClient,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := (&SakuraAuthPolicyReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	return nil
}
