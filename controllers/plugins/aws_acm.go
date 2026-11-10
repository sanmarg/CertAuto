package plugins

import (
	"context"
	"errors"
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

// Sync imports the certificate into AWS ACM and returns its ARN. When an ARN
// is already known it re-imports into that certificate, which updates it in
// place instead of creating a second one on every rotation.
func (p *AWSACMPlugin) Sync(
	ctx context.Context,
	secret *corev1.Secret,
	destConfig certautov1.DestinationConfig,
) (string, error) {
	logger := log.FromContext(ctx)

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(destConfig.Region))
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %v", err)
	}
	acmClient := acm.NewFromConfig(cfg)

	certBytes := secret.Data["tls.crt"]
	keyBytes := secret.Data["tls.key"]
	chainBytes := secret.Data["ca.crt"]

	if destConfig.CertificateARN != "" {
		exists, existsErr := p.CheckExists(ctx, destConfig)
		if existsErr != nil {
			return "", existsErr
		}
		if exists {
			logger.Info("Re-importing certificate into ACM", "arn", destConfig.CertificateARN)
			output, importErr := acmClient.ImportCertificate(ctx, &acm.ImportCertificateInput{
				Certificate:      certBytes,
				PrivateKey:       keyBytes,
				CertificateChain: chainBytes,
				CertificateArn:   aws.String(destConfig.CertificateARN),
			})
			if importErr != nil {
				return "", fmt.Errorf("failed to update ACM certificate %s: %w", destConfig.CertificateARN, importErr)
			}
			return aws.ToString(output.CertificateArn), nil
		}
	}

	logger.Info("Importing new ACM certificate")
	output, err := acmClient.ImportCertificate(ctx, &acm.ImportCertificateInput{
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
	if err != nil {
		return "", fmt.Errorf("failed to import ACM certificate: %w", err)
	}

	arn := aws.ToString(output.CertificateArn)
	if arn == "" {
		return "", fmt.Errorf("ACM import succeeded but returned no certificate ARN")
	}
	return arn, nil
}

// CheckExists checks if the certificate exists in ACM. Only a genuine
// not-found answer means "absent"; permission and throttling errors are
// returned so they are not mistaken for an empty account.
func (p *AWSACMPlugin) CheckExists(
	ctx context.Context,
	destConfig certautov1.DestinationConfig,
) (bool, error) {
	if destConfig.CertificateARN == "" {
		return false, nil
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(destConfig.Region))
	if err != nil {
		return false, fmt.Errorf("failed to load AWS config: %w", err)
	}
	acmClient := acm.NewFromConfig(cfg)

	_, err = acmClient.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(destConfig.CertificateARN),
	})
	if err != nil {
		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to describe ACM certificate %s: %w", destConfig.CertificateARN, err)
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
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	acmClient := acm.NewFromConfig(cfg)

	_, err = acmClient.DeleteCertificate(ctx, &acm.DeleteCertificateInput{
		CertificateArn: aws.String(destConfig.CertificateARN),
	})
	if err != nil {
		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("failed to delete ACM certificate %s: %w", destConfig.CertificateARN, err)
	}
	return nil
}
