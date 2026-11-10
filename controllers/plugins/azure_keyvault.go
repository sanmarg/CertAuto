package plugins

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	certautov1 "github.com/sanmarg/certauto/api/v1"
)

// AzureKeyVaultPlugin syncs certificates to Azure Key Vault.
type AzureKeyVaultPlugin struct {
	client.Client
}

// Name returns the plugin name.
func (p *AzureKeyVaultPlugin) Name() string {
	return "AzureKeyVault"
}

// Sync imports the certificate into Azure Key Vault and returns the
// certificate name used in the vault, so a later reconcile updates the same
// certificate instead of guessing a different name.
func (p *AzureKeyVaultPlugin) Sync(
	ctx context.Context,
	secret *corev1.Secret,
	destConfig certautov1.DestinationConfig,
) (string, error) {
	logger := log.FromContext(ctx)

	if destConfig.KeyVaultName == "" {
		return "", fmt.Errorf("keyVaultName is required for AzureKeyVault destinations")
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", fmt.Errorf("failed to authenticate: %v", err)
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", destConfig.KeyVaultName)
	certClient, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create KeyVault client: %v", err)
	}

	certName := destConfig.CertificateName
	if certName == "" {
		certName = generateCertName(secret)
	}

	config := destConfig
	config.CertificateName = certName
	exists, err := p.CheckExists(ctx, config)
	if err != nil {
		return "", err
	}

	certBytes := secret.Data["tls.crt"]
	keyBytes := secret.Data["tls.key"]
	base64Cert := base64.StdEncoding.EncodeToString(combinedPEM(certBytes, keyBytes))

	if exists {
		logger.Info("Importing new certificate version", "certificate", certName)
	} else {
		logger.Info("Creating new certificate", "certificate", certName)
	}

	if _, err := certClient.ImportCertificate(ctx, certName, azcertificates.ImportCertificateParameters{
		Base64EncodedCertificate: &base64Cert,
	}, nil); err != nil {
		return "", fmt.Errorf("failed to import certificate into Key Vault: %w", err)
	}

	return certName, nil
}

// CheckExists checks if the certificate exists in Key Vault. Only a 404 is
// treated as absence; authentication and permission failures are returned.
func (p *AzureKeyVaultPlugin) CheckExists(
	ctx context.Context,
	destConfig certautov1.DestinationConfig,
) (bool, error) {
	if destConfig.KeyVaultName == "" || destConfig.CertificateName == "" {
		return false, nil
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return false, err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", destConfig.KeyVaultName)
	certClient, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return false, err
	}

	_, err = certClient.GetCertificate(ctx, destConfig.CertificateName, "", nil)
	if err != nil {
		var responseError *azcore.ResponseError
		if errors.As(err, &responseError) && responseError.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete deletes the certificate from Key Vault.
func (p *AzureKeyVaultPlugin) Delete(
	ctx context.Context,
	destConfig certautov1.DestinationConfig,
) error {
	if destConfig.KeyVaultName == "" || destConfig.CertificateName == "" {
		return nil
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", destConfig.KeyVaultName)
	certClient, err := azcertificates.NewClient(vaultURL, cred, nil)
	if err != nil {
		return err
	}

	_, err = certClient.DeleteCertificate(ctx, destConfig.CertificateName, nil)
	if err != nil {
		var responseError *azcore.ResponseError
		if errors.As(err, &responseError) && responseError.StatusCode == 404 {
			return nil
		}
		return err
	}
	return nil
}

// combinedPEM concatenates a certificate and its private key without touching
// the backing arrays of the inputs. append(certBytes, keyBytes...) looked
// harmless but could write key material into the cached secret's spare
// capacity.
func combinedPEM(certBytes, keyBytes []byte) []byte {
	combined := make([]byte, 0, len(certBytes)+len(keyBytes)+1)
	combined = append(combined, certBytes...)
	if len(certBytes) > 0 && certBytes[len(certBytes)-1] != '\n' {
		combined = append(combined, '\n')
	}
	combined = append(combined, keyBytes...)
	return combined
}

// generateCertName generates a certificate name from the secret metadata.
func generateCertName(secret *corev1.Secret) string {
	return fmt.Sprintf("%s-%s", secret.Namespace, secret.Name)
}
