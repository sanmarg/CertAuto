package plugins

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	certautov1 "github.com/sanmargp/certauto/api/v1"
)

// KubernetesReflectorPlugin reflects/copies TLS secrets to target namespaces.
// This is useful when cert-manager creates certificates in a central namespace
// and you need to sync them to application namespaces.
type KubernetesReflectorPlugin struct {
	client.Client
}

// Name returns the plugin name.
func (p *KubernetesReflectorPlugin) Name() string {
	return "Kubernetes"
}

// Sync copies the TLS secret to the target namespace.
func (p *KubernetesReflectorPlugin) Sync(ctx context.Context, sourceSecret *corev1.Secret, destConfig certautov1.DestinationConfig) error {
	logger := log.FromContext(ctx)

	targetNamespace := destConfig.TargetNamespace
	targetSecretName := destConfig.TargetSecretName

	// Use source secret name if target name not specified
	if targetSecretName == "" {
		targetSecretName = sourceSecret.Name
	}

	if targetNamespace == "" {
		return fmt.Errorf("targetNamespace is required for Kubernetes reflector")
	}

	// Check if target namespace exists
	ns := &corev1.Namespace{}
	if err := p.Get(ctx, types.NamespacedName{Name: targetNamespace}, ns); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("target namespace %s does not exist", targetNamespace)
		}
		return fmt.Errorf("failed to check target namespace: %v", err)
	}

	// Check if target secret already exists
	existingSecret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      targetSecretName,
		Namespace: targetNamespace,
	}

	exists := true
	if err := p.Get(ctx, secretKey, existingSecret); err != nil {
		if errors.IsNotFound(err) {
			exists = false
		} else {
			return fmt.Errorf("failed to check existing secret: %v", err)
		}
	}

	// Create or update the target secret
	targetSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetSecretName,
			Namespace: targetNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":           "certauto",
				"certauto.sanorg.in/source-name":      sourceSecret.Name,
				"certauto.sanorg.in/source-namespace": sourceSecret.Namespace,
			},
			Annotations: map[string]string{
				"certauto.sanorg.in/reflected-from": fmt.Sprintf("%s/%s", sourceSecret.Namespace, sourceSecret.Name),
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": sourceSecret.Data["tls.crt"],
			"tls.key": sourceSecret.Data["tls.key"],
		},
	}

	// Copy ca.crt if present
	if caCrt, ok := sourceSecret.Data["ca.crt"]; ok {
		targetSecret.Data["ca.crt"] = caCrt
	}

	if exists {
		// Check if data has changed
		if secretDataEqual(existingSecret.Data, targetSecret.Data) {
			logger.Info("Secret data unchanged, skipping update",
				"targetNamespace", targetNamespace,
				"targetSecret", targetSecretName)
			return nil
		}

		// Update existing secret
		existingSecret.Data = targetSecret.Data
		existingSecret.Labels = targetSecret.Labels
		existingSecret.Annotations = targetSecret.Annotations

		logger.Info("Updating reflected secret",
			"targetNamespace", targetNamespace,
			"targetSecret", targetSecretName)

		if err := p.Update(ctx, existingSecret); err != nil {
			return fmt.Errorf("failed to update secret: %v", err)
		}
	} else {
		// Create new secret
		logger.Info("Creating reflected secret",
			"targetNamespace", targetNamespace,
			"targetSecret", targetSecretName)

		if err := p.Create(ctx, targetSecret); err != nil {
			return fmt.Errorf("failed to create secret: %v", err)
		}
	}

	return nil
}

// CheckExists checks if the secret exists in the target namespace.
func (p *KubernetesReflectorPlugin) CheckExists(ctx context.Context, destConfig certautov1.DestinationConfig) (bool, error) {
	targetNamespace := destConfig.TargetNamespace
	targetSecretName := destConfig.TargetSecretName

	if targetNamespace == "" || targetSecretName == "" {
		return false, nil
	}

	secret := &corev1.Secret{}
	err := p.Get(ctx, types.NamespacedName{
		Name:      targetSecretName,
		Namespace: targetNamespace,
	}, secret)

	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// DeleteSecret removes the reflected secret from the target namespace.
func (p *KubernetesReflectorPlugin) Delete(ctx context.Context, destConfig certautov1.DestinationConfig) error {
	logger := log.FromContext(ctx)

	targetNamespace := destConfig.TargetNamespace
	targetSecretName := destConfig.TargetSecretName

	if targetNamespace == "" || targetSecretName == "" {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetSecretName,
			Namespace: targetNamespace,
		},
	}

	logger.Info("Deleting reflected secret",
		"targetNamespace", targetNamespace,
		"targetSecret", targetSecretName)

	if err := p.Client.Delete(ctx, secret); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete secret: %v", err)
	}

	return nil
}

// secretDataEqual compares two secret data maps for equality.
func secretDataEqual(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for key, valA := range a {
		valB, ok := b[key]
		if !ok {
			return false
		}
		if string(valA) != string(valB) {
			return false
		}
	}
	return true
}
