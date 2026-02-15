package controllers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func createTestCert(priv *rsa.PrivateKey, notAfter time.Time) ([]byte, error) {
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return certPem, nil
}

func TestValidateTLSSecret(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	otherPriv, _ := rsa.GenerateKey(rand.Reader, 2048)

	validCert, _ := createTestCert(priv, time.Now().Add(24*time.Hour))
	expiredCert, _ := createTestCert(priv, time.Now().Add(-24*time.Hour))
	otherCert, _ := createTestCert(otherPriv, time.Now().Add(24*time.Hour))

	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	r := &CertificateBindingReconciler{}

	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr bool
	}{
		{
			name: "Valid certificate and key",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"tls.crt": validCert,
					"tls.key": keyBytes,
				},
			},
			wantErr: false,
		},
		{
			name: "Expired certificate",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"tls.crt": expiredCert,
					"tls.key": keyBytes,
				},
			},
			wantErr: true,
		},
		{
			name: "Mismatched key",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"tls.crt": otherCert,
					"tls.key": keyBytes,
				},
			},
			wantErr: true,
		},
		{
			name: "Missing crt",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"tls.key": keyBytes,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.validateTLSSecret(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTLSSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
