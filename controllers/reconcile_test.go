package controllers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	certautov1 "github.com/sanmarg/certauto/api/v1"
	"github.com/sanmarg/certauto/controllers/plugins"
)

const (
	testSourceNamespace = "cert-manager"
	testTargetNamespace = "apps"
	testSecretName      = "payments-tls"
	testBindingName     = "payments"
)

func newTestReconciler(t *testing.T, objects ...client.Object) *CertificateBindingReconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding core scheme: %v", err)
	}
	if err := certautov1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding certauto scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&certautov1.CertificateBinding{}).
		Build()

	return &CertificateBindingReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: scheme,
		plugins: map[string]DestinationPlugin{
			"Kubernetes": &plugins.KubernetesReflectorPlugin{Client: fakeClient},
		},
	}
}

func testTLSSecret(notAfter time.Time) *corev1.Secret {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	certPEM, err := createTestCert(priv, notAfter)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: testSourceNamespace},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
		},
	}
}

func testBinding(
	destinations ...certautov1.DestinationRule,
) *certautov1.CertificateBinding {
	return &certautov1.CertificateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testBindingName,
			Namespace:  testSourceNamespace,
			Generation: 1,
		},
		Spec: certautov1.CertificateBindingSpec{
			SourceSecretRef: &certautov1.SecretRef{
				Name:      testSecretName,
				Namespace: testSourceNamespace,
			},
			DestinationRules: destinations,
		},
	}
}

func kubernetesDestination() certautov1.DestinationRule {
	return certautov1.DestinationRule{
		Name: testTargetNamespace,
		Type: "Kubernetes",
		Config: certautov1.DestinationConfig{
			TargetNamespace:  testTargetNamespace,
			TargetSecretName: testSecretName,
		},
	}
}

func requestFor(binding *certautov1.CertificateBinding) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{
		Name:      binding.Name,
		Namespace: binding.Namespace,
	}}
}

func targetSecretKey() types.NamespacedName {
	return types.NamespacedName{Name: testSecretName, Namespace: testTargetNamespace}
}

func TestReconcileCreatesReflectedSecret(t *testing.T) {
	ctx := context.Background()
	source := testTLSSecret(time.Now().Add(24 * time.Hour))
	binding := testBinding(kubernetesDestination())
	targetNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   testTargetNamespace,
		Labels: map[string]string{NamespaceOptInLabel: testSourceNamespace},
	}}

	reconciler := newTestReconciler(t, source, binding, targetNamespace)
	result, err := reconciler.Reconcile(ctx, requestFor(binding))
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("unexpected requeue: %v", result.RequeueAfter)
	}

	var reflected corev1.Secret
	if err := reconciler.Get(ctx, targetSecretKey(), &reflected); err != nil {
		t.Fatalf("reading reflected secret: %v", err)
	}
	if !bytes.Equal(reflected.Data["tls.crt"], source.Data["tls.crt"]) {
		t.Fatal("reflected certificate does not match the source")
	}
	if reflected.Labels["app.kubernetes.io/managed-by"] != "certauto" {
		t.Fatalf("reflected secret is not marked as managed by certauto: %v", reflected.Labels)
	}

	var updated certautov1.CertificateBinding
	if err := reconciler.Get(ctx, requestFor(binding).NamespacedName, &updated); err != nil {
		t.Fatalf("reading binding: %v", err)
	}
	if !updated.Status.Ready {
		t.Fatalf("expected Ready, got status %+v", updated.Status)
	}
	if len(updated.Status.Destinations) != 1 || updated.Status.Destinations[0].State != certautov1.SyncStateSynced {
		t.Fatalf("unexpected destination status: %+v", updated.Status.Destinations)
	}
	if updated.Status.Destinations[0].ResourceID == "" {
		t.Fatal("expected the destination resource id to be persisted")
	}
	if !contains(updated.Finalizers, FinalizerName) {
		t.Fatalf("expected finalizer %q, got %v", FinalizerName, updated.Finalizers)
	}
}

func TestReconcileRejectsCrossNamespaceWithoutOptIn(t *testing.T) {
	ctx := context.Background()
	source := testTLSSecret(time.Now().Add(24 * time.Hour))
	binding := testBinding(kubernetesDestination())
	targetNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testTargetNamespace}}

	reconciler := newTestReconciler(t, source, binding, targetNamespace)
	if _, err := reconciler.Reconcile(ctx, requestFor(binding)); err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	var reflected corev1.Secret
	err := reconciler.Get(ctx, targetSecretKey(), &reflected)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected no reflected secret, got err=%v", err)
	}

	var updated certautov1.CertificateBinding
	if err := reconciler.Get(ctx, requestFor(binding).NamespacedName, &updated); err != nil {
		t.Fatalf("reading binding: %v", err)
	}
	if updated.Status.Ready {
		t.Fatal("binding should not be Ready when the target namespace has not opted in")
	}
	if len(updated.Status.Destinations) != 1 {
		t.Fatalf("unexpected destinations: %+v", updated.Status.Destinations)
	}
	if updated.Status.Destinations[0].State != certautov1.SyncStateFailed {
		t.Fatalf("expected Failed, got %s", updated.Status.Destinations[0].State)
	}
	if !strings.Contains(updated.Status.Destinations[0].Error, NamespaceOptInLabel) {
		t.Fatalf("error should mention the opt-in label, got %q", updated.Status.Destinations[0].Error)
	}
}

func TestReconcileDoesNotPropagateExpiredCertificate(t *testing.T) {
	ctx := context.Background()
	source := testTLSSecret(time.Now().Add(-time.Hour))
	binding := testBinding(kubernetesDestination())
	targetNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   testTargetNamespace,
		Labels: map[string]string{NamespaceOptInLabel: "*"},
	}}

	reconciler := newTestReconciler(t, source, binding, targetNamespace)
	if _, err := reconciler.Reconcile(ctx, requestFor(binding)); err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	var reflected corev1.Secret
	err := reconciler.Get(ctx, targetSecretKey(), &reflected)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expired certificate must not be propagated, got err=%v", err)
	}

	var updated certautov1.CertificateBinding
	if err := reconciler.Get(ctx, requestFor(binding).NamespacedName, &updated); err != nil {
		t.Fatalf("reading binding: %v", err)
	}
	if updated.Status.Ready {
		t.Fatal("binding with an expired certificate should not be Ready")
	}
}

func TestRunOnceSkipsUnchangedSource(t *testing.T) {
	ctx := context.Background()
	source := testTLSSecret(time.Now().Add(24 * time.Hour))
	binding := testBinding(kubernetesDestination())
	binding.Spec.SyncPolicy.RunOnce = true
	targetNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   testTargetNamespace,
		Labels: map[string]string{NamespaceOptInLabel: testSourceNamespace},
	}}

	reconciler := newTestReconciler(t, source, binding, targetNamespace)
	if _, err := reconciler.Reconcile(ctx, requestFor(binding)); err != nil {
		t.Fatalf("first reconcile returned error: %v", err)
	}

	var reflected corev1.Secret
	if err := reconciler.Get(ctx, targetSecretKey(), &reflected); err != nil {
		t.Fatalf("reading reflected secret: %v", err)
	}
	reflected.Data["tls.crt"] = []byte("edited by hand")
	if err := reconciler.Update(ctx, &reflected); err != nil {
		t.Fatalf("editing reflected secret: %v", err)
	}

	if _, err := reconciler.Reconcile(ctx, requestFor(binding)); err != nil {
		t.Fatalf("second reconcile returned error: %v", err)
	}

	var afterSecond corev1.Secret
	if err := reconciler.Get(ctx, targetSecretKey(), &afterSecond); err != nil {
		t.Fatalf("reading reflected secret: %v", err)
	}
	if !bytes.Equal(afterSecond.Data["tls.crt"], []byte("edited by hand")) {
		t.Fatal("runOnce should not re-sync when the source secret is unchanged")
	}
}

type failingPlugin struct {
	name string
}

func (p *failingPlugin) Name() string { return p.name }

func (p *failingPlugin) Sync(
	context.Context,
	*corev1.Secret,
	certautov1.DestinationConfig,
) (string, error) {
	return "", errors.New("cloud unavailable")
}

func (p *failingPlugin) CheckExists(context.Context, certautov1.DestinationConfig) (bool, error) {
	return false, nil
}

func (p *failingPlugin) Delete(context.Context, certautov1.DestinationConfig) error { return nil }

func TestSyncPolicyRetriesThenFails(t *testing.T) {
	ctx := context.Background()
	source := testTLSSecret(time.Now().Add(24 * time.Hour))
	binding := testBinding(
		certautov1.DestinationRule{Name: "acm", Type: "AWSACM"})
	binding.Spec.SyncPolicy = certautov1.SyncPolicy{MaxRetries: 2, RetryInterval: "1s"}

	reconciler := newTestReconciler(t, source, binding)
	reconciler.plugins["AWSACM"] = &failingPlugin{name: "AWSACM"}

	result, err := reconciler.Reconcile(ctx, requestFor(binding))
	if err != nil {
		t.Fatalf("first reconcile returned error: %v", err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("expected a 1s retry, got %v", result.RequeueAfter)
	}

	var afterFirst certautov1.CertificateBinding
	if err := reconciler.Get(ctx, requestFor(binding).NamespacedName, &afterFirst); err != nil {
		t.Fatalf("reading binding: %v", err)
	}
	if got := afterFirst.Status.Destinations[0]; got.State != certautov1.SyncStateRetrying || got.RetryCount != 1 {
		t.Fatalf("unexpected first attempt status: %+v", got)
	}

	result, err = reconciler.Reconcile(ctx, requestFor(binding))
	if err != nil {
		t.Fatalf("second reconcile returned error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected retries to stop, got requeue %v", result.RequeueAfter)
	}

	var afterSecond certautov1.CertificateBinding
	if err := reconciler.Get(ctx, requestFor(binding).NamespacedName, &afterSecond); err != nil {
		t.Fatalf("reading binding: %v", err)
	}
	if got := afterSecond.Status.Destinations[0]; got.State != certautov1.SyncStateFailed || got.RetryCount != 2 {
		t.Fatalf("unexpected second attempt status: %+v", got)
	}
}

func TestFinalizeRemovesDestinationAndFinalizer(t *testing.T) {
	ctx := context.Background()
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSecretName,
			Namespace: testTargetNamespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "certauto"},
		},
		Type: corev1.SecretTypeTLS,
	}
	now := metav1.Now()
	binding := testBinding(kubernetesDestination())
	binding.DeletionTimestamp = &now
	binding.Finalizers = []string{FinalizerName}

	reconciler := newTestReconciler(t, target, binding)
	var stored certautov1.CertificateBinding
	if err := reconciler.Get(ctx, requestFor(binding).NamespacedName, &stored); err != nil {
		t.Fatalf("reading binding: %v", err)
	}
	if _, err := reconciler.finalizeBinding(ctx, &stored); err != nil {
		t.Fatalf("finalize returned error: %v", err)
	}

	var reflected corev1.Secret
	err := reconciler.Get(ctx, targetSecretKey(), &reflected)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected the reflected secret to be deleted, got err=%v", err)
	}

	var remaining certautov1.CertificateBinding
	err = reconciler.Get(ctx, requestFor(binding).NamespacedName, &remaining)
	if err == nil {
		if contains(remaining.Finalizers, FinalizerName) {
			t.Fatalf("finalizer was not removed: %v", remaining.Finalizers)
		}
		return
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("unexpected error reading binding: %v", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
