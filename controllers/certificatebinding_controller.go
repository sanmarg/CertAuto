package controllers

import (
	"context"
	"fmt"
	"time"

	"crypto/tls"
	"crypto/x509"
	"encoding/pem"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certautov1 "github.com/sanmargp/certauto/api/v1"
	custommetrics "github.com/sanmargp/certauto/controllers/metrics"
	"github.com/sanmargp/certauto/controllers/plugins"
)

// CertificateBindingReconciler reconciles CertificateBinding objects
type CertificateBindingReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	// Plugin registry
	plugins map[string]DestinationPlugin
}

// DestinationPlugin interface for all destination plugins
type DestinationPlugin interface {
	Name() string
	Sync(ctx context.Context, secret *corev1.Secret, config certautov1.DestinationConfig) error
	CheckExists(ctx context.Context, config certautov1.DestinationConfig) (bool, error)
	Delete(ctx context.Context, config certautov1.DestinationConfig) error
}

// +kubebuilder:rbac:groups=sanorg.in,resources=certificatebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sanorg.in,resources=certificatebindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sanorg.in,resources=certificatebindings/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete

func (r *CertificateBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("certificatebinding", req.NamespacedName)

	// 1. Fetch the CertificateBinding instance
	var binding certautov1.CertificateBinding
	if err := r.Get(ctx, req.NamespacedName, &binding); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 2. Handle cert-manager Certificate management if configured
	sourceSecretName := ""
	sourceSecretNamespace := ""

	if binding.Spec.Certificate != nil {
		certName := binding.Name
		secretName := binding.Spec.Certificate.SecretName
		if secretName == "" {
			secretName = fmt.Sprintf("%s-tls", binding.Name)
		}
		sourceSecretName = secretName
		sourceSecretNamespace = binding.Namespace

		// Ensure Certificate exists
		err := r.ensureCertificate(ctx, &binding, certName, secretName)
		if err != nil {
			log.Error(err, "Failed to ensure cert-manager Certificate")
			return r.updateStatusWithError(ctx, &binding, fmt.Sprintf("Certificate failed: %v", err))
		}
	} else if binding.Spec.SourceSecretRef != nil {
		sourceSecretName = binding.Spec.SourceSecretRef.Name
		sourceSecretNamespace = binding.Spec.SourceSecretRef.Namespace
	} else {
		return r.updateStatusWithError(ctx, &binding, "Neither Certificate nor SourceSecretRef provided")
	}

	// 3. Fetch Source Secret
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: sourceSecretName, Namespace: sourceSecretNamespace}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Source secret not yet available", "secret", sourceSecretName)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return r.updateStatusWithError(ctx, &binding, fmt.Sprintf("Failed to get secret: %v", err))
	}

	if secret.Type != corev1.SecretTypeTLS {
		return r.updateStatusWithError(ctx, &binding, "Secret is not of type kubernetes.io/tls")
	}

	// 3.5 Validate Certificate
	if err := r.validateTLSSecret(secret); err != nil {
		log.Error(err, "TLS Certificate validation failed")
		custommetrics.ValidationFailedTotal.WithLabelValues(binding.Namespace, binding.Name, "invalid_tls").Inc()
		return r.updateStatusWithError(ctx, &binding, fmt.Sprintf("Certificate validation failed: %v", err))
	}

	// 4. Process Destinations
	allSynced := true
	var destStatuses []certautov1.DestinationStatus

	for _, dest := range binding.Spec.DestinationRules {
		destStatus := certautov1.DestinationStatus{
			Name: dest.Name,
			Type: dest.Type,
		}

		plugin, exists := r.plugins[dest.Type]
		if !exists {
			destStatus.State = certautov1.SyncStateFailed
			destStatus.Error = fmt.Sprintf("Unknown destination type: %s", dest.Type)
			destStatuses = append(destStatuses, destStatus)
			allSynced = false
			custommetrics.SyncTotal.WithLabelValues(dest.Type, "error").Inc()
			continue
		}

		startTime := time.Now()
		if binding.Spec.DryRun {
			log.Info("[DRY-RUN] Would sync certificate to destination", "destination", dest.Name, "type", dest.Type)
			destStatus.State = certautov1.SyncStateSynced
			destStatus.Error = "Dry Run: No action taken"
			now := metav1.Now()
			destStatus.LastSync = &now
		} else if err := plugin.Sync(ctx, secret, dest.Config); err != nil {
			destStatus.State = certautov1.SyncStateFailed
			destStatus.Error = err.Error()
			allSynced = false
			custommetrics.SyncTotal.WithLabelValues(dest.Type, "error").Inc()
		} else {
			destStatus.State = certautov1.SyncStateSynced
			now := metav1.Now()
			destStatus.LastSync = &now
			custommetrics.SyncTotal.WithLabelValues(dest.Type, "success").Inc()
			custommetrics.SyncDuration.WithLabelValues(dest.Type).Observe(time.Since(startTime).Seconds())

			// Record expiry metric
			if expiry, err := getCertExpiry(secret); err == nil {
				custommetrics.CertificateExpirySeconds.WithLabelValues(binding.Namespace, binding.Name, dest.Name).Set(float64(expiry.Unix()))
			}
		}
		destStatuses = append(destStatuses, destStatus)
	}

	// 5. Update Status
	binding.Status.Destinations = destStatuses
	binding.Status.Ready = allSynced
	binding.Status.SyncCount++
	now := metav1.Now()
	binding.Status.LastSyncTime = &now
	binding.Status.ObservedGeneration = binding.Generation

	if binding.Spec.DryRun {
		meta.SetStatusCondition(&binding.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "DryRun",
			Message: "Dry run successful: operations simulated",
		})
	} else if allSynced {
		meta.SetStatusCondition(&binding.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "AllDestinationsSynced",
			Message: "All destination sync operations succeeded",
		})
	} else {
		meta.SetStatusCondition(&binding.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  "SyncFailed",
			Message: "One or more destination sync operations failed",
		})
	}

	if err := r.Status().Update(ctx, &binding); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CertificateBindingReconciler) validateTLSSecret(secret *corev1.Secret) error {
	certData, ok := secret.Data["tls.crt"]
	if !ok {
		return fmt.Errorf("missing tls.crt in secret")
	}
	keyData, ok := secret.Data["tls.key"]
	if !ok {
		return fmt.Errorf("missing tls.key in secret")
	}

	// 1. Try to load the key-pair to verify match
	_, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return fmt.Errorf("certificate and key do not match: %v", err)
	}

	// 2. Verify certificate is not expired
	block, _ := pem.Decode(certData)
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %v", err)
	}

	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("certificate is expired (expired on %s)", cert.NotAfter)
	}

	return nil
}

func getCertExpiry(secret *corev1.Secret) (time.Time, error) {
	certData := secret.Data["tls.crt"]
	block, _ := pem.Decode(certData)
	if block == nil {
		return time.Time{}, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return cert.NotAfter, nil
}

func (r *CertificateBindingReconciler) ensureCertificate(ctx context.Context, binding *certautov1.CertificateBinding, certName, secretName string) error {
	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certName,
			Namespace: binding.Namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, cert, func() error {
		cert.Spec = certmanagerv1.CertificateSpec{
			SecretName: secretName,
			DNSNames:   binding.Spec.Certificate.DNSNames,
			CommonName: binding.Spec.Certificate.CommonName,
			IssuerRef: cmmeta.ObjectReference{
				Name:  binding.Spec.Certificate.IssuerRef.Name,
				Kind:  binding.Spec.Certificate.IssuerRef.Kind,
				Group: binding.Spec.Certificate.IssuerRef.Group,
			},
			Duration:    binding.Spec.Certificate.Duration,
			RenewBefore: binding.Spec.Certificate.RenewBefore,
		}
		if cert.Spec.IssuerRef.Kind == "" {
			cert.Spec.IssuerRef.Kind = "Issuer"
		}
		if cert.Spec.IssuerRef.Group == "" {
			cert.Spec.IssuerRef.Group = "cert-manager.io"
		}

		return ctrl.SetControllerReference(binding, cert, r.Scheme)
	})

	return err
}

func (r *CertificateBindingReconciler) updateStatusWithError(ctx context.Context, binding *certautov1.CertificateBinding, errorMsg string) (ctrl.Result, error) {
	binding.Status.Ready = false
	meta.SetStatusCondition(&binding.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  "ReconcileError",
		Message: errorMsg,
	})
	if err := r.Status().Update(ctx, binding); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *CertificateBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.plugins = make(map[string]DestinationPlugin)
	r.plugins["AzureKeyVault"] = &plugins.AzureKeyVaultPlugin{Client: r.Client}
	r.plugins["AWSACM"] = &plugins.AWSACMPlugin{Client: r.Client}
	r.plugins["Kubernetes"] = &plugins.KubernetesReflectorPlugin{Client: r.Client}

	return ctrl.NewControllerManagedBy(mgr).
		For(&certautov1.CertificateBinding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&certmanagerv1.Certificate{}).
		Owns(&corev1.Secret{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapSecretToBinding),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *CertificateBindingReconciler) mapSecretToBinding(ctx context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok || secret.Type != corev1.SecretTypeTLS {
		return nil
	}

	var list certautov1.CertificateBindingList
	if err := r.List(ctx, &list, client.InNamespace(secret.Namespace)); err != nil {
		return nil
	}

	var requests []ctrl.Request
	for _, b := range list.Items {
		// If managed certificate
		if b.Spec.Certificate != nil {
			secretName := b.Spec.Certificate.SecretName
			if secretName == "" {
				secretName = fmt.Sprintf("%s-tls", b.Name)
			}
			if secret.Name == secretName {
				requests = append(requests, ctrl.Request{NamespacedName: types.NamespacedName{Name: b.Name, Namespace: b.Namespace}})
			}
		} else if b.Spec.SourceSecretRef != nil && b.Spec.SourceSecretRef.Name == secret.Name {
			requests = append(requests, ctrl.Request{NamespacedName: types.NamespacedName{Name: b.Name, Namespace: b.Namespace}})
		}
	}
	return requests
}
