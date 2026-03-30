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
	AnnotationServiceID     = "gateway.sakura.io/service-id"
	AnnotationRouteIDs      = "gateway.sakura.io/route-ids"
	AnnotationDomainIDs     = "gateway.sakura.io/domain-ids"
	AnnotationCertificateIDs = "gateway.sakura.io/certificate-ids"
	AnnotationNodePortService = "gateway.sakura.io/nodeport-service"
	AnnotationCreatedSubscription = "gateway.sakura.io/created-subscription"

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
