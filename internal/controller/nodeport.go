package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// NodePortManager manages NodePort Services for HTTPRoute backends.
type NodePortManager struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// NodePortResult contains the result of ensuring a NodePort Service.
type NodePortResult struct {
	NodePort   int32
	ExternalIP string
	ServiceName string
}

// EnsureNodePortService creates or updates a NodePort Service for the given backend.
func (m *NodePortManager) EnsureNodePortService(ctx context.Context, owner metav1.Object, backendName string, backendPort int32, namespace string) (*NodePortResult, error) {
	npServiceName := fmt.Sprintf("%s-sakura-gw-np", backendName)

	// Fetch the original ClusterIP service to get its selector
	var originalSvc corev1.Service
	if err := m.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: backendName}, &originalSvc); err != nil {
		return nil, fmt.Errorf("get backend service %s: %w", backendName, err)
	}

	if originalSvc.Spec.Selector == nil || len(originalSvc.Spec.Selector) == 0 {
		return nil, fmt.Errorf("backend service %s has no selector", backendName)
	}

	// Check if NodePort service already exists
	var npSvc corev1.Service
	err := m.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: npServiceName}, &npSvc)
	if err == nil {
		// Already exists, return the NodePort
		for _, port := range npSvc.Spec.Ports {
			if port.Port == backendPort {
				externalIP, err := m.getNodeExternalIP(ctx)
				if err != nil {
					return nil, err
				}
				return &NodePortResult{
					NodePort:    port.NodePort,
					ExternalIP:  externalIP,
					ServiceName: npServiceName,
				}, nil
			}
		}
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("get nodeport service: %w", err)
	}

	// Create NodePort service
	npSvc = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      npServiceName,
			Namespace: namespace,
			Labels: map[string]string{
				LabelManagedBy: LabelManagedByValue,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: originalSvc.Spec.Selector,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       backendPort,
					TargetPort: intstr.FromInt32(backendPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// Set owner reference if owner is in the same namespace
	if ownerGW, ok := owner.(*gatewayv1.HTTPRoute); ok {
		if err := ctrl.SetControllerReference(ownerGW, &npSvc, m.Scheme); err != nil {
			return nil, fmt.Errorf("set owner reference: %w", err)
		}
	}

	if err := m.Client.Create(ctx, &npSvc); err != nil {
		return nil, fmt.Errorf("create nodeport service: %w", err)
	}

	// Re-read to get allocated NodePort
	if err := m.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: npServiceName}, &npSvc); err != nil {
		return nil, fmt.Errorf("re-read nodeport service: %w", err)
	}

	var nodePort int32
	for _, port := range npSvc.Spec.Ports {
		if port.Port == backendPort {
			nodePort = port.NodePort
			break
		}
	}

	externalIP, err := m.getNodeExternalIP(ctx)
	if err != nil {
		return nil, err
	}

	return &NodePortResult{
		NodePort:    nodePort,
		ExternalIP:  externalIP,
		ServiceName: npServiceName,
	}, nil
}

// DeleteNodePortService deletes the managed NodePort Service.
func (m *NodePortManager) DeleteNodePortService(ctx context.Context, backendName, namespace string) error {
	npServiceName := fmt.Sprintf("%s-sakura-gw-np", backendName)
	var npSvc corev1.Service
	err := m.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: npServiceName}, &npSvc)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// Only delete if we own it
	if npSvc.Labels[LabelManagedBy] != LabelManagedByValue {
		return nil
	}

	return m.Client.Delete(ctx, &npSvc)
}

// getNodeExternalIP returns the ExternalIP of the first Ready node.
// Falls back to InternalIP if no ExternalIP is available.
func (m *NodePortManager) getNodeExternalIP(ctx context.Context) (string, error) {
	var nodeList corev1.NodeList
	if err := m.Client.List(ctx, &nodeList); err != nil {
		return "", fmt.Errorf("list nodes: %w", err)
	}

	for _, node := range nodeList.Items {
		// Check if node is Ready
		isReady := false
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}
		if !isReady {
			continue
		}

		// Prefer ExternalIP, fall back to InternalIP
		var internalIP string
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeExternalIP && addr.Address != "" {
				return addr.Address, nil
			}
			if addr.Type == corev1.NodeInternalIP && addr.Address != "" {
				internalIP = addr.Address
			}
		}
		if internalIP != "" {
			return internalIP, nil
		}
	}

	return "", fmt.Errorf("no ready node with IP address found")
}
