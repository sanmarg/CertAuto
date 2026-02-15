package plugins

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acm/types"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	certautov1 "github.com/sanmarg/certauto/api/v1"
)

// AWSACMPlugin syncs certificates to AWS ACM.
type AWSACMPlugin struct {
	client.Client
}

// Name returns the plugin name.
func (p *AWSACMPlugin) Name() string {
	return "AWSACM"
}

// Sync syncs the certificate to AWS ACM.
func (p *AWSACMPlugin) Sync(ctx context.Context, secret *corev1.Secret, destConfig certautov1.DestinationConfig) error {
	logger := log.FromContext(ctx)

	// 1. Load AWS config
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(destConfig.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	// 2. Create ACM client
	acmClient := acm.NewFromConfig(cfg)

	// 3. Check if certificate exists
	exists, err := p.CheckExists(ctx, destConfig)
	if err != nil {
		return err
	}

	// 4. Import or update certificate
	certBytes := secret.Data["tls.crt"]
	keyBytes := secret.Data["tls.key"]
	chainBytes := secret.Data["ca.crt"]

	if exists && destConfig.CertificateARN != "" {
		// Re-import certificate to update it
		logger.Info("Importing certificate to ACM", "arn", destConfig.CertificateARN)
		_, err = acmClient.ImportCertificate(ctx, &acm.ImportCertificateInput{
			Certificate:      certBytes,
			PrivateKey:       keyBytes,
			CertificateChain: chainBytes,
			CertificateArn:   aws.String(destConfig.CertificateARN),
		})
	} else {
		// Import new certificate
		logger.Info("Importing new ACM certificate")
		_, err = acmClient.ImportCertificate(ctx, &acm.ImportCertificateInput{
			Certificate:      certBytes,
			PrivateKey:       keyBytes,
			CertificateChain: chainBytes,
			Tags: []types.Tag{
				{
					Key:   aws.String("ManagedBy"),
					Value: aws.String("certauto"),
				},
			},
		})
	}

	return err
}

// CheckExists checks if the certificate exists in ACM.
func (p *AWSACMPlugin) CheckExists(ctx context.Context, destConfig certautov1.DestinationConfig) (bool, error) {
	if destConfig.CertificateARN == "" {
		return false, nil
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(destConfig.Region))
	if err != nil {
		return false, err
	}

	acmClient := acm.NewFromConfig(cfg)

	_, err = acmClient.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(destConfig.CertificateARN),
	})
	if err != nil {
		// Check if it's a not found error
		return false, nil
	}
	return true, nil
}

// Delete deletes the certificate from ACM.
func (p *AWSACMPlugin) Delete(ctx context.Context, destConfig certautov1.DestinationConfig) error {
	if destConfig.CertificateARN == "" {
		return nil
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(destConfig.Region))
	if err != nil {
		return err
	}

	acmClient := acm.NewFromConfig(cfg)

	_, err = acmClient.DeleteCertificate(ctx, &acm.DeleteCertificateInput{
		CertificateArn: aws.String(destConfig.CertificateARN),
	})
	return err
}

// getDomainFromSecret extracts the domain from the TLS certificate.
func getDomainFromSecret(secret *corev1.Secret) string {
	// TODO: Parse the certificate to extract the domain
	return ""
}
