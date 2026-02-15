<p align="center">
  <img src="docs/assets/logo.png" width="200" alt="CertAuto Logo">
</p>

# CertAuto — Certificate Auto-Renewal & Distribution Controller

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.24+-blue.svg)](https://kubernetes.io)

CertAuto is a Kubernetes controller that automates TLS certificate renewal and distribution. It integrates with cert-manager to create/manage Certificates and syncs resulting TLS secrets to configured destinations (Kubernetes namespaces, Azure Key Vault, AWS ACM).

This README is written for production operators and contributors — it covers architecture, deployment, configuration, development workflow, and security considerations.

## Table of Contents

- Overview
- Architecture (diagram)
- Installation (Helm)
- Configuration
- Usage & Examples
- Observability & Metrics
- Security & Secrets
- Development: build, test, codegen, lint
- CI / Release notes
- Contributing
- Troubleshooting
- License

## Overview

CertAuto watches a custom resource `CertificateBinding` (group `sanorg.in`) and ensures certificates are created (via cert-manager) or consumed from existing secrets, validated, and synchronized to the destinations defined in the `CertificateBinding` spec. Plugins drive destination integrations: Kubernetes Reflector, Azure Key Vault, and AWS ACM.

## Architecture

<img width="1293" height="345" alt="image" src="https://github.com/user-attachments/assets/e19657ce-339a-4240-813a-df47f2554479" />

Core design goals:

- Single source of truth: `CertificateBinding` CRs declare certificate lifecycle and destinations.
- Idempotent sync: repeated reconciles are safe and use metadata labels/annotations to track reflected secrets.
- Safe-by-default: validation prevents propagating invalid/expired certs; dry-run mode for testing.

## Installation (Production)

1. Ensure prerequisites:

  - Kubernetes 1.24+
  - cert-manager installed and functioning
  - Helm 3

2. Install the chart:

```bash
# from repository root
helm install certauto ./charts/certauto -n certauto-system --create-namespace
```

3. Verify controller pods are running:

```bash
kubectl get pods -n certauto-system
kubectl logs -n certauto-system -l app.kubernetes.io/name=certauto
```

4. Configure cloud credentials as Kubernetes Secrets (examples):

- Azure: create `azure-credentials` secret referenced by `config/manager/manager.yaml` (`tenantId`, `clientId`, `clientSecret`).
- AWS: provide IAM role for service account or secrets per your cloud best practices.

Do not store long-lived cloud keys in the repo.

## Configuration

- CRD: `config/crd/bases/sanorg.in_certificatebindings.yaml`
- Sample manifests: `config/samples/*.yaml`
- Controller deployment: `config/manager/manager.yaml` (images, env vars, secretKeyRefs)

Key config notes:

- The controller expects TLS secrets to follow Kubernetes TLS secret format (`tls.crt`, `tls.key`).
- Plugins accept per-destination `config` fields in `CertificateBinding.spec.destinationRules`.

## Usage & Examples

Apply an example binding from `config/samples`:

```bash
kubectl apply -f config/samples/certificatebinding_kubernetes.yaml
kubectl get certificatebindings -A
kubectl describe certificatebinding <name> -n <namespace>
```

Typical flow:

1. Create `CertificateBinding` CR.
2. Controller ensures cert-manager `Certificate` created (if configured).
3. Secret is produced by cert-manager, controller validates TLS contents.
4. Controller syncs to destinations and updates `status.destinations`.

## Observability & Metrics

The controller exposes Prometheus metrics (configurable via flags and `config/default`). Important metrics:

- `certauto_sync_total{destination,type,result}`
- `certauto_sync_duration_seconds`
- `certauto_certificate_expiry_seconds`
- `certauto_validation_failed_total`

Scrape endpoint default options are defined in `config/default/metrics_service.yaml` and `config/prometheus` overlays.

## Security & Secrets

- Do NOT commit secrets or private keys to git. Search for `tls.key`, `kubeconfig`, `clientSecret` before committing.
- Use Kubernetes `Secret` resources and RBAC to grant least privilege.
- For cloud credentials prefer Workload Identity (GCP), MSI (Azure), or IRSA (AWS) instead of static keys.

I scanned the repository for common secret patterns and did not find plaintext private keys or cloud secrets. The controller references external secrets (e.g., `azure-credentials`) via `secretKeyRef` — these should be created in-cluster by operators/CI.

## Development

Prerequisites (local):

- Go 1.20+ (or project-specified Go version)
- `make`, `helm`, `kubectl`
- `golangci-lint` for linting

Common tasks:

- Build

```bash
make build
```

- Run unit tests

```bash
go test ./... -v
```

- Lint (requires golangci-lint)

```bash
golangci-lint run
```

- Code generation (when API types change)

```bash
make generate
```

- Run the controller locally against a cluster:

```bash
# use manager image or run `go run ./cmd` with proper kubeconfig
go run ./cmd -metrics-bind-address=:8080
```

Testing note: the repository includes e2e tests under `test/e2e`. Use an envtest cluster or KinD for CI.

## CI and Releases

- CI should run `go test ./...`, `golangci-lint run`, and `make generate` as part of PR validation.
- Releases should build and publish container images, update Helm `Chart.yaml`, and publish chart artifacts.

## Contributing

We welcome fixes and improvements. Please follow these guidelines:

1. Fork and create a feature branch.
2. Write tests for new behavior and ensure all tests pass.
3. Run linters and format code (`gofmt`).
4. Open a PR with a clear description and reference to issues.

Contributor checklist:

```bash
go test ./...
golangci-lint run
gofmt -s -w .
```

If you want, I can add a `CONTRIBUTING.md` with these guidelines and a PR template.

## Troubleshooting

- Check controller logs:

```bash
kubectl logs -n certauto-system -l app.kubernetes.io/name=certauto
```

- Inspect a specific `CertificateBinding`:

```bash
kubectl describe certificatebinding <name> -n <namespace>
```

- Common issues:
  - Missing `tls.crt`/`tls.key` in source secret — verify cert-manager output or source secret presence.
  - Cloud plugin authentication failures — verify secrets/roles and plugin config.

## License

Apache 2.0 — see LICENSE file.

---

If you'd like, I will:

- Create a `CONTRIBUTING.md` and PR template.
- Add a short `docs/ARCHITECTURE.md` expanding the mermaid diagram with sequence flows.

Tell me which of those you'd like next.
