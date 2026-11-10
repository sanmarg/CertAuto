package plugins

import (
	"bytes"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateCertName(t *testing.T) {
	tests := []struct {
		name   string
		secret *corev1.Secret
		want   string
	}{
		{
			name: "Standard names",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
			},
			want: "default-test-secret",
		},
		{
			name: "Different namespace",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "api-certs",
					Namespace: "prod",
				},
			},
			want: "prod-api-certs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateCertName(tt.secret); got != tt.want {
				t.Errorf("generateCertName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCombinedPEMDoesNotMutateCertBytes(t *testing.T) {
	// Give the certificate slice spare capacity: append(cert, key...) would
	// write into that capacity and corrupt the source secret's backing array.
	certBytes := make([]byte, 4, 32)
	copy(certBytes, []byte("cert"))
	keyBytes := []byte("key")

	combined := combinedPEM(certBytes, keyBytes)

	if !bytes.Equal(certBytes, []byte("cert")) {
		t.Fatalf("source certificate bytes were mutated: %q", certBytes)
	}
	spare := certBytes[:cap(certBytes)]
	for i := len(certBytes); i < len(spare); i++ {
		if spare[i] != 0 {
			t.Fatalf("append wrote into the source slice's spare capacity at %d", i)
		}
	}
	if !bytes.Equal(combined, []byte("cert\nkey")) {
		t.Fatalf("unexpected combined PEM: %q", combined)
	}
}
