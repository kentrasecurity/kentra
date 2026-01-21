package services

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	securityv1alpha1 "github.com/kentrasecurity/kentra/api/v1alpha1"
)

// ReconcileReverseShellService creates a LoadBalancer service for reverse shell connections
func ReconcileReverseShellService(ctx context.Context, c client.Client, exploit *securityv1alpha1.Exploit) error {
	log := log.FromContext(ctx)

	serviceName := fmt.Sprintf("%s-revshell", exploit.Name)
	namespace := exploit.Namespace

	// Parse and validate port
	port, err := strconv.ParseInt(exploit.Spec.ReverseShell.Port, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid port format: %w", err)
	}
	if errs := validation.IsValidPortNum(int(port)); len(errs) > 0 {
		return fmt.Errorf("port %d is out of range: %v", port, errs)
	}

	// Check if service already exists
	svc := &corev1.Service{}
	err = c.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, svc)
	if err == nil {
		// Service exists
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Create new service
	newService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                     exploit.Name,
				"kentra.sh/resource-type": "exploit",
				"kentra.sh/reverse-shell": "true",
			},
			Annotations: map[string]string{
				"kentra.sh/exploit-name": exploit.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: exploit.APIVersion,
					Kind:       exploit.Kind,
					Name:       exploit.Name,
					UID:        exploit.UID,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"job-name": exploit.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "revshell",
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(port),
					TargetPort: intstr.FromInt(int(port)),
				},
			},
		},
	}

	if err := c.Create(ctx, newService); err != nil {
		log.Error(err, "Failed to create reverse shell service", "service", serviceName)
		return err
	}

	log.Info("Created reverse shell LoadBalancer service", "service", serviceName, "port", port)
	return nil
}
