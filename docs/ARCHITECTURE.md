# Architecture — CertAuto

This document expands the high-level architecture diagram and shows the primary sequence flow for certificate issuance and distribution.

## Components

- `CertificateBinding` CR (group `sanorg.in`): declares the certificate lifecycle and destination rules.
- Controller (CertAuto): watches `CertificateBinding` CRs, orchestrates cert creation/consumption, validates TLS secrets, and syncs to destinations via plugins.
- cert-manager: issues certificates and creates Kubernetes TLS secrets.
- Plugins: Kubernetes Reflector, Azure Key Vault, AWS ACM.
- Prometheus metrics and leader election (coordination.k8s.io/leases).

## Sequence flow

1. Operator creates a `CertificateBinding` CR (or via GitOps/helm).
2. Controller reconciler picks up the CR and determines source (managed Certificate vs `sourceSecretRef`).
3. If managed: controller creates a `cert-manager` `Certificate` resource.
4. cert-manager obtains/renews certificate and writes a TLS Secret (`tls.crt`, `tls.key`).
5. Controller reads the TLS Secret and validates certificate + key match and expiry.
6. Controller executes configured plugins:
   - Kubernetes Reflector: creates/updates target Secret(s) in other namespaces and sets labels/annotations for traceability.
   - AzureKeyVault: imports certificate material into Key Vault.
   - AWSACM: imports certificate into AWS Certificate Manager.
7. Controller updates `CertificateBinding.status.destinations` with sync results.

## Failure handling

- Validation fails: controller sets status to indicate validation failure and will not sync.
- Plugin sync fails: controller increments retry counters and follows `syncPolicy` (maxRetries, retryInterval).

## Observability

- Metrics emitted: sync counts, durations, validation failures, expiry timestamps.
- Logs include structured fields: `namespace`, `certificatebinding`, `destination`, and error details.

## Security considerations

- Do not store secrets in source control. Use Kubernetes `Secret` resources and cloud-native identity providers instead of embedding keys.
- Limit RBAC to least privilege for controller service account.
