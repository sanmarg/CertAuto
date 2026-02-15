/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncState represents the state of a sync operation.
type SyncState string

const (
	SyncStateSynced   SyncState = "Synced"
	SyncStateFailed   SyncState = "Failed"
	SyncStateError    SyncState = "Error"
	SyncStateRetrying SyncState = "Retrying"
	SyncStatePending  SyncState = "Pending"
)

// DestinationConfig defines the configuration for a certificate destination.
type DestinationConfig struct {
	// KeyVaultName is the name of the Azure Key Vault (for AzureKeyVault type).
	// +optional
	KeyVaultName string `json:"keyVaultName,omitempty"`

	// CertificateName is the name to use for the certificate in the destination.
	// +optional
	CertificateName string `json:"certificateName,omitempty"`

	// CertificateARN is the ARN of the ACM certificate (for AWSACM type).
	// +optional
	CertificateARN string `json:"certificateArn,omitempty"`

	// Region is the AWS region (for AWSACM type).
	// +optional
	Region string `json:"region,omitempty"`

	// TargetNamespace is the target namespace (for Kubernetes type).
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// TargetSecretName is the target secret name (for Kubernetes type).
	// +optional
	TargetSecretName string `json:"targetSecretName,omitempty"`
}

// DestinationRule defines a destination where certificates should be synced.
type DestinationRule struct {
	// Name is a unique identifier for this destination.
	Name string `json:"name"`

	// Type is the type of destination (AzureKeyVault, AWSACM, Kubernetes).
	Type string `json:"type"`

	// Config contains destination-specific configuration.
	Config DestinationConfig `json:"config"`
}

// SyncPolicy defines the sync policy for the certificate binding.
type SyncPolicy struct {
	// MaxRetries is the maximum number of retries before giving up.
	// +optional
	MaxRetries int32 `json:"maxRetries,omitempty"`

	// RetryInterval is the interval between retries.
	// +optional
	RetryInterval string `json:"retryInterval,omitempty"`

	// RunOnce if true, the controller will not re-sync after a successful operation until the source changes.
	// +optional
	RunOnce bool `json:"runOnce,omitempty"`
}

// DestinationStatus defines the status of a destination sync.
type DestinationStatus struct {
	// Name is the name of the destination.
	Name string `json:"name"`

	// Type is the type of destination.
	Type string `json:"type"`

	// State is the current state of the sync.
	State SyncState `json:"state"`

	// LastSync is the timestamp of the last successful sync.
	// +optional
	LastSync *metav1.Time `json:"lastSync,omitempty"`

	// Error contains any error message from the last sync attempt.
	// +optional
	Error string `json:"error,omitempty"`

	// RetryCount is the number of retry attempts.
	// +optional
	RetryCount int32 `json:"retryCount,omitempty"`
}

// IssuerRef references a cert-manager Issuer or ClusterIssuer.
type IssuerRef struct {
	// Name of the issuer.
	Name string `json:"name"`
	// Kind of the issuer (Issuer or ClusterIssuer).
	// +optional
	Kind string `json:"kind,omitempty"`
	// Group of the issuer.
	// +optional
	Group string `json:"group,omitempty"`
}

// CertificateSpec defines the cert-manager certificate to be managed.
type CertificateSpec struct {
	// DNSNames is a list of DNS names to include in the certificate.
	DNSNames []string `json:"dnsNames"`

	// IssuerRef is a reference to the issuer for this certificate.
	IssuerRef IssuerRef `json:"issuerRef"`

	// CommonName is the common name for the certificate.
	// +optional
	CommonName string `json:"commonName,omitempty"`

	// SecretName is the name of the secret to create (defaults to binding name-tls).
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// Duration is the lifetime of the certificate.
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// RenewBefore is the time before expiry to renew.
	// +optional
	RenewBefore *metav1.Duration `json:"renewBefore,omitempty"`
}

// SecretRef references a Kubernetes secret.
type SecretRef struct {
	// Name of the secret.
	Name string `json:"name"`
	// Namespace of the secret.
	Namespace string `json:"namespace"`
}

// CertificateBindingSpec defines the desired state of CertificateBinding.
type CertificateBindingSpec struct {
	// Certificate defines the configuration for a cert-manager Certificate.
	// If provided, the controller will create and manage a cert-manager Certificate.
	// +optional
	Certificate *CertificateSpec `json:"certificate,omitempty"`

	// SourceSecretRef is used if you already have a secret and don't want the controller to manage a Certificate.
	// +optional
	SourceSecretRef *SecretRef `json:"sourceSecretRef,omitempty"`

	// DestinationRules defines where to sync the certificate.
	// +optional
	DestinationRules []DestinationRule `json:"destinationRules,omitempty"`

	// DryRun if true, the controller will only simulate operations and log intentions.
	// +optional
	DryRun bool `json:"dryRun,omitempty"`

	// SyncPolicy defines the sync policy.
	// +optional
	SyncPolicy SyncPolicy `json:"syncPolicy,omitempty"`
}

// CertificateBindingStatus defines the observed state of CertificateBinding.
type CertificateBindingStatus struct {
	// Conditions represent the latest available observations of the CertificateBinding's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Ready indicates if the certificate binding is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Destinations contains the status of each destination.
	// +optional
	Destinations []DestinationStatus `json:"destinations,omitempty"`

	// LastSyncTime is the timestamp of the last sync operation.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// SyncCount is the number of times the controller has synced.
	// +optional
	SyncCount int64 `json:"syncCount,omitempty"`

	// ObservedGeneration is the latest generation of the CertificateBinding that was processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CertificateBinding is the Schema for the certificatebindings API.
type CertificateBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CertificateBindingSpec   `json:"spec,omitempty"`
	Status CertificateBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CertificateBindingList contains a list of CertificateBinding.
type CertificateBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CertificateBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CertificateBinding{}, &CertificateBindingList{})
}
