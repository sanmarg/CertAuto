# CertAuto: Certificate Auto-Renewal & Distribution Controller

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.24+-blue.svg)](https://kubernetes.io)

CertAuto is a Kubernetes controller that distributes TLS certificates. It works
with cert-manager, which issues and renews certificates, and copies the resulting
TLS secret to the places that need it: other namespaces, AWS ACM, or Azure Key
Vault.

One custom resource declares what the certificate is and where it has to exist.
The controller reconciles the difference.

## How it works

```text
CertificateBinding (sanorg.in/v1)
        |
        v
cert-manager Certificate  -->  TLS Secret (tls.crt / tls.key)
                                     |
                          validate keypair and expiry
                                     |
                 +-------------------+-------------------+
                 |                   |                   |
          Kubernetes copy       AWS ACM import     Azure Key Vault
```

The reconcile loop:

1. Read the `CertificateBinding`. If it is being deleted, clean up destinations
   and then remove the finalizer.
2. If `spec.certificate` is set, create or update a cert-manager `Certificate`
   owned by the binding. Otherwise read `spec.sourceSecretRef`.
3. Wait for the TLS secret. A missing secret requeues after 10 seconds.
4. Validate the keypair and expiry before anything leaves the cluster.
5. Sync every destination and record one status row per destination. A provider
   identifier (ACM ARN, Key Vault certificate name) is persisted in
   `status.destinations[].resourceID` so the next reconcile updates the existing
   object instead of creating a duplicate.
6. Write status with conflict retries, including a `Ready` condition and
   `observedGeneration`.

Destination plugins implement four methods: `Name`, `Sync`, `CheckExists` and
`Delete`. Available types are `Kubernetes`, `AWSACM` and `AzureKeyVault`.

## Requirements

- Kubernetes 1.24 or newer
- cert-manager installed if you use `spec.certificate`
- Helm 3 for the chart, or `kubectl` plus `make` for a kustomize install
- Go 1.25 to build from source

## Install with Helm

```bash
helm install certauto ./charts/certauto \
  -n certauto-system --create-namespace \
  --set image.repository=<your-registry>/certauto \
  --set image.tag=<version>
```

The chart is in `charts/certauto`. It does not create the CRD on upgrade, which
is standard Helm behaviour; install the chart before creating bindings.

To use workload identity instead of static credentials, annotate the service
account:

```bash
helm install certauto ./charts/certauto -n certauto-system --create-namespace \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=arn:aws:iam::123456789012:role/certauto
```

## Install with kustomize

```bash
make install    # CRDs
make deploy IMG=<your-registry>/certauto:<version>
```

## Usage

```yaml
apiVersion: sanorg.in/v1
kind: CertificateBinding
metadata:
  name: payments-api
  namespace: cert-manager
spec:
  certificate:
    dnsNames:
      - payments.internal.example
    issuerRef:
      name: internal-ca
      kind: ClusterIssuer
  destinationRules:
    - name: payments
      type: Kubernetes
      config:
        targetNamespace: payments
        targetSecretName: payments-tls
    - name: load-balancer
      type: AWSACM
      config:
        region: ap-south-1
```

```bash
kubectl apply -f binding.yaml
kubectl get certificatebindings -A
kubectl describe certificatebinding payments-api -n cert-manager
```

See `config/samples/` for Kubernetes, AWS, Azure and multi-destination examples.

### Cross-namespace opt-in

A binding may only write a TLS secret into another namespace when that namespace
consents. The receiving namespace must carry:

```yaml
metadata:
  labels:
    certauto.sanorg.in/allow-binding-from: cert-manager
```

The value is either a comma-separated list of source namespaces or `*`. Writes
inside the binding's own namespace do not need the label. Without it, the
destination is marked `Failed` and nothing is written.

This is a workload-level guard, not a replacement for RBAC. The controller's
ClusterRole can read secrets across the cluster because that is what reflection
requires.

### Sync policy

`spec.syncPolicy` is enforced:

- `maxRetries`: attempts per destination before it is marked `Failed`. Zero means
  one attempt per reconcile.
- `retryInterval`: Go duration between attempts, for example `30s`. Defaults to
  `30s`.
- `runOnce`: skip destination syncs when the binding is `Ready` and the source
  secret content has not changed since the last successful reconcile.

### Dry run

`spec.dryRun: true` validates the certificate and reports what would happen. It
does not call any destination. Destination rows are `Pending` and the `Ready`
condition is `False` with reason `DryRun`, because nothing was actually synced.

### Deletion

A finalizer (`certauto.sanorg.in/finalizer`) holds the binding until every
destination has been cleaned up. Reflected secrets are deleted, ACM certificates
are deleted, and Key Vault certificates are deleted. If cleanup fails, the
finalizer stays and the reconcile retries.

## Observability

The controller exports Prometheus metrics:

| Metric | Labels |
| --- | --- |
| `certauto_sync_total` | `destination_type`, `status` |
| `certauto_sync_duration_seconds` | `destination_type` |
| `certauto_certificate_expiry_seconds` | `namespace`, `name`, `destination` |
| `certauto_validation_failed_total` | `namespace`, `name`, `reason` |

`certauto_certificate_expiry_seconds` stores the expiry as a Unix timestamp, so
the useful alert is arithmetic:

```promql
time() - certauto_certificate_expiry_seconds < 1209600
```

That fires when a certificate has less than 14 days left.

## Security

- Certificate and key material is never written to status or logs.
- The Kubernetes destination copies both `tls.crt` and `tls.key` into the target
  namespace. This is a deliberate trade-off: the copy is the feature, and every
  copy is also another place the key can leak from. Prefer issuing per namespace
  or distributing only CA bundles when the workload allows it.
- Cross-namespace writes require the receiving namespace to opt in, as described
  above.
- AWS and Azure plugins use the default cloud credential chain. Prefer IRSA,
  Azure Workload Identity or managed identity over static keys. If you must use a
  static Azure secret, put it in `azure-credentials` with `tenantId`, `clientId`
  and `clientSecret` keys.
- The CRD rejects unknown destination types at admission. It does not validate
  that `targetNamespace`, `keyVaultName` or `region` match the destination type;
  those are checked by the controller at reconcile time.

## Limitations

These are known and intentional for the current version:

- No workload reload. Updating a secret does not restart pods or make an ingress
  controller reload it. That is the workload's responsibility.
- No multi-cluster distribution. One controller serves one cluster.
- The Kubernetes plugin copies private keys. There is no `IssuePerNamespace` or
  `CABundleOnly` mode yet.
- No admission webhook. Validation is limited to CRD enum and minimum-length
  rules plus reconcile-time checks.
- The API group is `sanorg.in`. Change it before publishing a fork unless you
  control that domain.
- AWS ACM import is not tested against LocalStack in CI. The e2e suite covers the
  controller lifecycle and metrics, not cloud calls.

## Development

```bash
make manifests generate   # CRD, RBAC and deepcopy
make fmt vet
make test                 # unit tests
make lint                 # golangci-lint
make test-e2e             # kind cluster, builds and loads the image
```

The unit tests cover certificate validation, the reconcile loop against a fake
client (including cross-namespace denial, expired certificates, retries and
`runOnce`), and the Key Vault PEM helper.

CI runs:

- `Tests`: unit tests on every push and pull request
- `Lint`: golangci-lint
- `E2E Tests`: kind cluster, image build and load, cert-manager install
- `Deploy Website`: GitHub Pages for `docs/`

## Contributing

1. Fork and create a feature branch.
2. Add tests for behaviour changes.
3. Run `go test ./...`, `golangci-lint run` and `gofmt -s -w .`.
4. Open a pull request with a clear description.

`CONTRIBUTING.md` has the longer version.

## Troubleshooting

```bash
kubectl logs -n certauto-system -l app.kubernetes.io/name=certauto
kubectl describe certificatebinding <name> -n <namespace>
```

Common causes:

- Missing `tls.crt` or `tls.key` in the source secret. Check cert-manager first.
- `target namespace has not opted in`: add the label described above.
- Cloud authentication failures: check the service account annotations and the
  provider's credential chain.
- A destination stuck in `Retrying`: read the per-destination error, then check
  the provider API status and quota.

## License

Apache 2.0. See `LICENSE`.
