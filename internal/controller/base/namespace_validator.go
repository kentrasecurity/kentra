package base

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ValidateNamespace checks if a namespace is managed by Kentra
func ValidateNamespace(ctx context.Context, c client.Client, namespace string) error {
	log := log.FromContext(ctx)

	ns := &corev1.Namespace{}
	if err := c.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		log.Error(err, "Failed to get namespace", "namespace", namespace)
		return err
	}

	if annotations := ns.GetAnnotations(); annotations != nil {
		if _, ok := annotations["managed-by-kentra"]; ok {
			return nil
		}
	}

	return fmt.Errorf("namespace %s is not managed by Kentra", namespace)
}
