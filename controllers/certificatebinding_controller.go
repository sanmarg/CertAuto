package controllers

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certautov1 "github.com/sanmarg/certauto/api/v1"
	custommetrics "github.com/sanmarg/certauto/controllers/metrics"
	"github.com/sanmarg/certauto/controllers/plugins"
)

const (
	// FinalizerName keeps a CertificateBinding around until every destination it
	// created has been cleaned up.
	FinalizerName = "certauto.sanorg.in/finalizer"

	// NamespaceOptInLabel must be present on a target namespace before a binding
	// from a different namespace may write a TLS secret into it. The value is
	// either "*" or a comma-separated list of source namespaces.
	NamespaceOptInLabel = "certauto.sanorg.in/allow-binding-from"

	destinationKubernetes = "Kubernetes"

	managedByLabel = "app.kubernetes.io/managed-by"
	managedByValue = "certauto"

	defaultRequeueAfter  = 10 * time.Second
	defaultRetryInterval = 30 * time.Second
)

// CertificateBindingReconciler reconciles CertificateBinding objects.
type CertificateBindingReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	// Plugin registry
	plugins map[string]DestinationPlugin
}

// DestinationPlugin is the interface every destination backend implements.
// Sync returns a provider-specific resource identifier (an ACM ARN, a Key Vault
// certificate name, or a namespace/name pair) which the controller persists in
// status so later reconciles update the same object instead of creating another.
type DestinationPlugin interface {
	Name() string
	Sync(ctx context.Context, secret *corev1.Secret, config certautov1.DestinationConfig) (string, error)
	CheckExists(ctx context.Context, config certautov1.DestinationConfig) (bool, error)
	Delete(ctx context.Context, config certautov1.DestinationConfig) error
}

// +kubebuilder:rbac:groups=sanorg.in,resources=certificatebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sanorg.in,resources=certificatebindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sanorg.in,resources=certificatebindings/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete

func (r *CertificateBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("certificatebinding", req.NamespacedName)

	// 1. Fetch the CertificateBinding instance.
	var binding certautov1.CertificateBinding
	if err := r.Get(ctx, req.NamespacedName, &binding); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 2. On deletion, clean up destinations before releasing the finalizer.
	if !binding.DeletionTimestamp.IsZero() {
		return r.finalizeBinding(ctx, &binding)
	}
	if !controllerutil.ContainsFinalizer(&binding, FinalizerName) {
		controllerutil.AddFinalizer(&binding, FinalizerName)
		if err := r.Update(ctx, &binding); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
	}

	// 3. Resolve the source secret, creating a cert-manager Certificate if the
	// spec asks for one.
	sourceSecretName, sourceSecretNamespace, err := r.resolveSourceSecret(ctx, &binding)
	if err != nil {
		return r.updateStatusWithError(ctx, req.NamespacedName, err.Error())
	}

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Name: sourceSecretName, Namespace: sourceSecretNamespace}
	if err := r.Get(ctx, secretKey, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Source secret not yet available", "secret", sourceSecretName)
			statusErr := r.patchStatus(ctx, req.NamespacedName, func(latest *certautov1.CertificateBinding) {
				latest.Status.Ready = false
				latest.Status.ObservedGeneration = latest.Generation
				meta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "WaitingForSourceSecret",
					Message:            fmt.Sprintf("source secret %s/%s not found", sourceSecretNamespace, sourceSecretName),
					ObservedGeneration: latest.Generation,
				})
			})
			if statusErr != nil {
				return ctrl.Result{}, statusErr
			}
			return ctrl.Result{RequeueAfter: defaultRequeueAfter}, nil
		}
		return r.updateStatusWithError(ctx, req.NamespacedName, fmt.Sprintf("failed to get secret: %v", err))
	}

	if secret.Type != corev1.SecretTypeTLS {
		return r.updateStatusWithError(ctx, req.NamespacedName, "secret is not of type kubernetes.io/tls")
	}

	// 4. Validate before anything is propagated.
	if err := r.validateTLSSecret(secret); err != nil {
		log.Error(err, "TLS certificate validation failed")
		custommetrics.ValidationFailedTotal.WithLabelValues(binding.Namespace, binding.Name, "invalid_tls").Inc()
		return r.updateStatusWithError(ctx, req.NamespacedName, fmt.Sprintf("certificate validation failed: %v", err))
	}

	secretHash := secretContentHash(secret)
	if r.runOnceSatisfied(&binding, secretHash) {
		log.V(1).Info("runOnce policy satisfied, skipping destination sync")
		return ctrl.Result{}, nil
	}

	// 5. Sync every destination and record one status row per destination.
	outcome := r.syncDestinations(ctx, &binding, secret)
	err = r.patchStatus(ctx, req.NamespacedName, func(latest *certautov1.CertificateBinding) {
		latest.Status.Destinations = outcome.destinations
		latest.Status.Ready = outcome.allSynced && !latest.Spec.DryRun
		latest.Status.SyncCount++
		now := metav1.Now()
		latest.Status.LastSyncTime = &now
		latest.Status.ObservedGeneration = latest.Generation
		latest.Status.SourceSecretHash = secretHash
		setReadyCondition(latest, outcome.allSynced, latest.Spec.DryRun)
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: outcome.requeueAfter}, nil
}

type syncOutcome struct {
	destinations []certautov1.DestinationStatus
	allSynced    bool
	requeueAfter time.Duration
}

func (r *CertificateBindingReconciler) syncDestinations(
	ctx context.Context,
	binding *certautov1.CertificateBinding,
	secret *corev1.Secret,
) syncOutcome {
	log := r.Log.WithValues("certificatebinding", binding.Namespace+"/"+binding.Name)
	outcome := syncOutcome{
		destinations: make([]certautov1.DestinationStatus, 0, len(binding.Spec.DestinationRules)),
		allSynced:    true,
	}

	for i := range binding.Spec.DestinationRules {
		dest := binding.Spec.DestinationRules[i]
		row := certautov1.DestinationStatus{Name: dest.Name, Type: dest.Type}
		previous := destinationStatus(binding, dest.Name)

		plugin, ok := r.plugins[dest.Type]
		if !ok {
			row.State = certautov1.SyncStateFailed
			row.Error = fmt.Sprintf("unknown destination type: %s", dest.Type)
			outcome.destinations = append(outcome.destinations, row)
			outcome.allSynced = false
			custommetrics.SyncTotal.WithLabelValues(dest.Type, "error").Inc()
			continue
		}

		config := effectiveConfig(dest, previous)

		if err := r.checkDestinationAccess(ctx, binding.Namespace, dest); err != nil {
			row.State = certautov1.SyncStateFailed
			row.Error = err.Error()
			outcome.destinations = append(outcome.destinations, row)
			outcome.allSynced = false
			custommetrics.SyncTotal.WithLabelValues(dest.Type, "error").Inc()
			continue
		}

		if binding.Spec.DryRun {
			log.Info("[DRY-RUN] would sync certificate", "destination", dest.Name, "type", dest.Type)
			row.State = certautov1.SyncStatePending
			row.Error = "dry run: no action taken"
			outcome.destinations = append(outcome.destinations, row)
			continue
		}

		start := time.Now()
		resourceID, syncErr := plugin.Sync(ctx, secret, config)
		if syncErr != nil {
			row.Error = syncErr.Error()
			row.RetryCount = nextRetryCount(previous)
			outcome.allSynced = false
			custommetrics.SyncTotal.WithLabelValues(dest.Type, "error").Inc()

			if retryAllowed(binding.Spec.SyncPolicy, row.RetryCount) {
				row.State = certautov1.SyncStateRetrying
				outcome.requeueAfter = earlier(outcome.requeueAfter, retryInterval(binding.Spec.SyncPolicy))
			} else {
				row.State = certautov1.SyncStateFailed
			}
			outcome.destinations = append(outcome.destinations, row)
			continue
		}

		now := metav1.Now()
		row.State = certautov1.SyncStateSynced
		row.ResourceID = resourceID
		row.LastSync = &now
		row.RetryCount = 0
		outcome.destinations = append(outcome.destinations, row)

		custommetrics.SyncTotal.WithLabelValues(dest.Type, "success").Inc()
		custommetrics.SyncDuration.WithLabelValues(dest.Type).Observe(time.Since(start).Seconds())

		if expiry, err := getCertExpiry(secret); err == nil {
			custommetrics.CertificateExpirySeconds.
				WithLabelValues(binding.Namespace, binding.Name, dest.Name).
				Set(float64(expiry.Unix()))
		}
	}

	return outcome
}

func destinationStatus(binding *certautov1.CertificateBinding, name string) *certautov1.DestinationStatus {
	for i := range binding.Status.Destinations {
		if binding.Status.Destinations[i].Name == name {
			return &binding.Status.Destinations[i]
		}
	}
	return nil
}

// effectiveConfig carries the persisted provider identifier back into the
// plugin so a re-sync updates the object it created earlier.
func effectiveConfig(
	dest certautov1.DestinationRule,
	previous *certautov1.DestinationStatus,
) certautov1.DestinationConfig {
	config := dest.Config
	if previous == nil || previous.ResourceID == "" {
		return config
	}
	switch dest.Type {
	case "AWSACM":
		if config.CertificateARN == "" {
			config.CertificateARN = previous.ResourceID
		}
	case "AzureKeyVault":
		if config.CertificateName == "" {
			config.CertificateName = previous.ResourceID
		}
	case destinationKubernetes:
		if config.TargetSecretName == "" {
			parts := strings.SplitN(previous.ResourceID, "/", 2)
			if len(parts) == 2 {
				config.TargetSecretName = parts[1]
			}
		}
	}
	return config
}

func nextRetryCount(previous *certautov1.DestinationStatus) int32 {
	if previous == nil {
		return 1
	}
	return previous.RetryCount + 1
}

func retryAllowed(policy certautov1.SyncPolicy, retryCount int32) bool {
	return policy.MaxRetries > 0 && retryCount < policy.MaxRetries
}

func retryInterval(policy certautov1.SyncPolicy) time.Duration {
	if policy.RetryInterval == "" {
		return defaultRetryInterval
	}
	interval, err := time.ParseDuration(policy.RetryInterval)
	if err != nil || interval <= 0 {
		return defaultRetryInterval
	}
	return interval
}

func earlier(current, candidate time.Duration) time.Duration {
	if current == 0 || candidate < current {
		return candidate
	}
	return current
}

func setReadyCondition(binding *certautov1.CertificateBinding, allSynced, dryRun bool) {
	condition := metav1.Condition{
		Type:               "Ready",
		ObservedGeneration: binding.Generation,
	}
	switch {
	case dryRun:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "DryRun"
		condition.Message = "Dry run: no changes applied"
	case allSynced:
		condition.Status = metav1.ConditionTrue
		condition.Reason = "AllDestinationsSynced"
		condition.Message = "All destination sync operations succeeded"
	default:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "SyncFailed"
		condition.Message = "One or more destination sync operations failed"
	}
	meta.SetStatusCondition(&binding.Status.Conditions, condition)
}

func (r *CertificateBindingReconciler) runOnceSatisfied(
	binding *certautov1.CertificateBinding,
	secretHash string,
) bool {
	policy := binding.Spec.SyncPolicy
	return policy.RunOnce &&
		binding.Status.Ready &&
		binding.Status.ObservedGeneration == binding.Generation &&
		binding.Status.SourceSecretHash == secretHash
}

func secretContentHash(secret *corev1.Secret) string {
	digest := sha256.New()
	digest.Write(secret.Data["tls.crt"])
	digest.Write([]byte{0})
	digest.Write(secret.Data["tls.key"])
	return hex.EncodeToString(digest.Sum(nil))
}

// checkDestinationAccess enforces receiving-side consent for cross-namespace
// writes. A namespace must carry the opt-in label naming the source namespace
// (or "*") before another namespace can put a TLS secret in it.
func (r *CertificateBindingReconciler) checkDestinationAccess(
	ctx context.Context,
	sourceNamespace string,
	dest certautov1.DestinationRule,
) error {
	if dest.Type != destinationKubernetes {
		return nil
	}
	target := dest.Config.TargetNamespace
	if target == "" {
		return fmt.Errorf("targetNamespace is required for Kubernetes destinations")
	}
	if target == sourceNamespace {
		return nil
	}

	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: target}, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("target namespace %q does not exist", target)
		}
		return fmt.Errorf("reading target namespace %q: %w", target, err)
	}

	allowed := namespace.Labels[NamespaceOptInLabel]
	if allowed == "*" {
		return nil
	}
	for _, candidate := range strings.Split(allowed, ",") {
		if strings.TrimSpace(candidate) == sourceNamespace {
			return nil
		}
	}
	return fmt.Errorf(
		"target namespace %q has not opted in to bindings from %q (set label %s)",
		target, sourceNamespace, NamespaceOptInLabel,
	)
}

func (r *CertificateBindingReconciler) resolveSourceSecret(
	ctx context.Context,
	binding *certautov1.CertificateBinding,
) (string, string, error) {
	if binding.Spec.Certificate != nil {
		secretName := binding.Spec.Certificate.SecretName
		if secretName == "" {
			secretName = fmt.Sprintf("%s-tls", binding.Name)
		}
		if err := r.ensureCertificate(ctx, binding, binding.Name, secretName); err != nil {
			return "", "", fmt.Errorf("ensuring cert-manager Certificate: %w", err)
		}
		return secretName, binding.Namespace, nil
	}
	if binding.Spec.SourceSecretRef != nil {
		return binding.Spec.SourceSecretRef.Name, sourceSecretNamespace(binding), nil
	}
	return "", "", fmt.Errorf("neither certificate nor sourceSecretRef is set")
}

// sourceSecretRef returns the name and namespace of the secret a binding reads.
func sourceSecretRef(binding *certautov1.CertificateBinding) (string, string) {
	if binding.Spec.Certificate != nil {
		name := binding.Spec.Certificate.SecretName
		if name == "" {
			name = fmt.Sprintf("%s-tls", binding.Name)
		}
		return name, binding.Namespace
	}
	if binding.Spec.SourceSecretRef != nil {
		return binding.Spec.SourceSecretRef.Name, sourceSecretNamespace(binding)
	}
	return "", ""
}

func sourceSecretNamespace(binding *certautov1.CertificateBinding) string {
	if binding.Spec.SourceSecretRef != nil && binding.Spec.SourceSecretRef.Namespace != "" {
		return binding.Spec.SourceSecretRef.Namespace
	}
	return binding.Namespace
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
	if _, err := tls.X509KeyPair(certData, keyData); err != nil {
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

func (r *CertificateBindingReconciler) ensureCertificate(
	ctx context.Context,
	binding *certautov1.CertificateBinding,
	certName, secretName string,
) error {
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

// patchStatus re-reads the binding and writes status with conflict retries, so
// a concurrent reconcile cannot silently drop the update.
func (r *CertificateBindingReconciler) patchStatus(
	ctx context.Context,
	key types.NamespacedName,
	mutate func(*certautov1.CertificateBinding),
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest certautov1.CertificateBinding
		if err := r.Get(ctx, key, &latest); err != nil {
			return err
		}
		mutate(&latest)
		return r.Status().Update(ctx, &latest)
	})
}

func (r *CertificateBindingReconciler) updateStatusWithError(
	ctx context.Context,
	key types.NamespacedName,
	errorMsg string,
) (ctrl.Result, error) {
	err := r.patchStatus(ctx, key, func(binding *certautov1.CertificateBinding) {
		binding.Status.Ready = false
		binding.Status.ObservedGeneration = binding.Generation
		meta.SetStatusCondition(&binding.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ReconcileError",
			Message:            errorMsg,
			ObservedGeneration: binding.Generation,
		})
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// finalizeBinding releases the destinations created for a binding, and only
// then removes the finalizer. A cleanup failure keeps the finalizer and backs
// off instead of leaking the destination silently.
func (r *CertificateBindingReconciler) finalizeBinding(
	ctx context.Context,
	binding *certautov1.CertificateBinding,
) (ctrl.Result, error) {
	log := r.Log.WithValues("certificatebinding", binding.Namespace+"/"+binding.Name)
	if !controllerutil.ContainsFinalizer(binding, FinalizerName) {
		return ctrl.Result{}, nil
	}

	var cleanupErrors []string
	for i := range binding.Spec.DestinationRules {
		dest := binding.Spec.DestinationRules[i]
		plugin, ok := r.plugins[dest.Type]
		if !ok {
			continue
		}
		config := effectiveConfig(dest, destinationStatus(binding, dest.Name))
		if err := plugin.Delete(ctx, config); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("%s: %v", dest.Name, err))
		}
	}

	if len(cleanupErrors) > 0 {
		message := strings.Join(cleanupErrors, "; ")
		log.Error(fmt.Errorf("%s", message), "cleanup incomplete, keeping finalizer")
		key := types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}
		_ = r.patchStatus(ctx, key, func(latest *certautov1.CertificateBinding) {
			latest.Status.Ready = false
			meta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "CleanupFailed",
				Message:            message,
				ObservedGeneration: latest.Generation,
			})
		})
		return ctrl.Result{RequeueAfter: defaultRetryInterval}, nil
	}

	controllerutil.RemoveFinalizer(binding, FinalizerName)
	if err := r.Update(ctx, binding); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CertificateBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.plugins = make(map[string]DestinationPlugin)
	r.plugins["AzureKeyVault"] = &plugins.AzureKeyVaultPlugin{Client: r.Client}
	r.plugins["AWSACM"] = &plugins.AWSACMPlugin{Client: r.Client}
	r.plugins[destinationKubernetes] = &plugins.KubernetesReflectorPlugin{Client: r.Client}

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

// mapSecretToBinding maps both the source secret and any reflected target
// secret back to the bindings that care about them. Watching targets is what
// corrects drift when someone edits a reflected secret by hand.
func (r *CertificateBindingReconciler) mapSecretToBinding(ctx context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok || secret.Type != corev1.SecretTypeTLS {
		return nil
	}

	var bindings certautov1.CertificateBindingList
	if err := r.List(ctx, &bindings); err != nil {
		return nil
	}

	isManagedTarget := secret.Labels[managedByLabel] == managedByValue
	seen := make(map[types.NamespacedName]struct{})
	var requests []ctrl.Request

	add := func(name, namespace string) {
		key := types.NamespacedName{Name: name, Namespace: namespace}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		requests = append(requests, ctrl.Request{NamespacedName: key})
	}

	for i := range bindings.Items {
		binding := &bindings.Items[i]
		if name, namespace := sourceSecretRef(binding); name == secret.Name && namespace == secret.Namespace {
			add(binding.Name, binding.Namespace)
		}
		if !isManagedTarget {
			continue
		}
		for _, dest := range binding.Spec.DestinationRules {
			if dest.Type != destinationKubernetes || dest.Config.TargetNamespace != secret.Namespace {
				continue
			}
			targetName := dest.Config.TargetSecretName
			if targetName == "" {
				targetName, _ = sourceSecretRef(binding)
			}
			if targetName == secret.Name {
				add(binding.Name, binding.Namespace)
				break
			}
		}
	}

	return requests
}
