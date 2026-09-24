# Open Banking Financial Insights Service

A production-shaped reference service for ingesting **mocked UK Open Banking account-information data**, categorising transactions asynchronously, and producing monthly financial insights. It never needs bank credentials or real customer data.

## Architecture

```text
Mock client -> Go API -> PostgreSQL
                    \-> Kafka transaction-ingested -> Go worker -> categories + monthly_insights
                         Prometheus / OpenTelemetry -> Grafana / trace backend
```

- The API accepts a sandbox payload only when its explicit consent is active and unexpired.
- PostgreSQL stores consents, accounts, transactions, summaries, and an append-only audit trail.
- Newly inserted (idempotent by `external_id`) transactions are published to Kafka.
- The worker categorises spending, rebuilds the relevant monthly summary, and expires stale consents.
- JSON logs, request IDs, `/health/live`, `/health/ready`, `/metrics`, and OTLP traces provide operational visibility.
- Docker Compose provides the local stack. Kubernetes and Terraform provide AWS-oriented deployment examples (EKS, RDS PostgreSQL, MSK Serverless, ECR).

This design is inspired by UK Open Banking account-information permissions and consent lifecycles, but is not a certified implementation of the security profile, FAPI, dynamic client registration, or OBIE conformance suite.

## Run locally

Requirements: Docker with Compose.

```bash
docker compose up --build -d
curl -X POST http://localhost:8080/v1/sandbox/ingest \
  -H 'Content-Type: application/json' \
  --data-binary @mockdata/sample.json
curl http://localhost:8080/v1/accounts/22222222-2222-4222-8222-222222222222/insights
```

The worker is asynchronous; allow a few seconds before reading insights. Useful endpoints:

- API: `http://localhost:8080`
- Grafana: `http://localhost:3000` (admin / sandbox; local only)
- Prometheus: `http://localhost:9090`
- Revoke consent: `POST /v1/consents/{consent-id}/revoke`

Run tests without a host Go installation:

```bash
make test
```

## Security and data handling

- Only synthetic sample identifiers and data are committed.
- Money is stored as integer minor units to avoid rounding errors.
- Consent state is checked before ingestion; revocation is immediate and expiry is enforced at request time plus periodically persisted.
- Container images run as non-root with a read-only filesystem in Kubernetes.
- Production secrets belong in AWS Secrets Manager/External Secrets, not ConfigMaps or Git.
- For a real provider integration, add mTLS, private-key JWT/JARM/FAPI controls, encrypted token storage, fine-grained scopes, signature validation, rate limiting, and independent compliance/security review.

## AWS deployment notes

The Terraform under `deploy/terraform/environments/dev` is a compact baseline. Configure a versioned S3 state backend, review cost and retention settings, and add NAT/endpoints and managed node groups or Fargate before applying. RDS generates its master password in Secrets Manager. MSK Serverless uses IAM authentication, so production clients need the AWS MSK IAM mechanism rather than the local plaintext Kafka settings.

CI tests and vets the Go code, lints the Dockerfile, then uses GitHub OIDC to build/push the API image on `main`. Configure `AWS_DEPLOY_ROLE` and `ECR_REGISTRY` as repository secrets; add a deployment approval job for production.

## API payload

`mockdata/sample.json` is a complete request for `POST /v1/sandbox/ingest`. Timestamps are RFC 3339 and amounts use pence (for GBP). Retrying the same payload does not duplicate transactions.
