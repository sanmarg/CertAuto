package plugins

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	certautov1 "github.com/sanmargp/certauto/api/v1"
)

// AzureKeyVaultPlugin syncs certificates to Azure Key Vault.
type AzureKeyVaultPlugin struct {
	client.Client
}

// Name returns the plugin name.
func (p *AzureKeyVaultPlugin) Name() string {
	return "AzureKeyVault"
}

// Sync syncs the certificate to Azure Key Vault.
func (p *AzureKeyVaultPlugin) Sync(ctx context.Context, secret *corev1.Secret, destConfig certautov1.DestinationConfig) error {
	logger := log.FromContext(ctx)

	// 1. Authenticate using Managed Identity
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("failed to authenticate: %v", err)
	}

	// 2. Create KeyVault client
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", destConfig.KeyVaultName)
	certClient, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create KeyVault client: %v", err)
	}

	// 3. Determine certificate name
	certName := destConfig.CertificateName
	if certName == "" {
		certName = generateCertName(secret)
	}

	// 4. Check if certificate exists
	exists, err := p.CheckExists(ctx, destConfig)
	if err != nil {
		return err
	}

	// 5. Prepare certificate data (combine cert and key into PFX/PEM format)
	certBytes := secret.Data["tls.crt"]
	keyBytes := secret.Data["tls.key"]

	// Combine cert and key for import (base64 encoded PEM)
	combinedPEM := append(certBytes, keyBytes...)
	base64Cert := base64.StdEncoding.EncodeToString(combinedPEM)

	// 6. Import certificate
	if exists {
		logger.Info("Importing new certificate version", "certificate", certName)
	} else {
		logger.Info("Creating new certificate", "certificate", certName)
	}

	_, err = certClient.ImportCertificate(ctx, certName, azcertificates.ImportCertificateParameters{
		Base64EncodedCertificate: &base64Cert,
	}, nil)

	return err
}

// CheckExists checks if the certificate exists in Key Vault.
func (p *AzureKeyVaultPlugin) CheckExists(ctx context.Context, destConfig certautov1.DestinationConfig) (bool, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return false, err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", destConfig.KeyVaultName)
	certClient, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return false, err
	}

	certName := destConfig.CertificateName
	_, err = certClient.GetCertificate(ctx, certName, "", nil)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "CertificateNotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete deletes the certificate from Key Vault.
func (p *AzureKeyVaultPlugin) Delete(ctx context.Context, destConfig certautov1.DestinationConfig) error {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", destConfig.KeyVaultName)
	certClient, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return err
	}

	certName := destConfig.CertificateName
	_, err = certClient.DeleteCertificate(ctx, certName, nil)
	return err
}

// generateCertName generates a certificate name from the secret metadata.
func generateCertName(secret *corev1.Secret) string {
	return fmt.Sprintf("%s-%s", secret.Namespace, secret.Name)
}
