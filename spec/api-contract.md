# API Contract — Webhook Endpoint

## Endpoint

```
POST /api/github-webhook
```

## Request

### Headers

| Header | Required | Description |
|--------|----------|-------------|
| `X-Hub-Signature-256` | Yes | HMAC-SHA256 signature of the request body using webhook secret |
| `X-GitHub-Event` | Yes | GitHub event type (e.g. `push`, `ping`) |
| `Content-Type` | Yes | `application/json` |

### Body

GitHub push event payload. Key fields used:

| Field | Type | Description |
|-------|------|-------------|
| `ref` | string | Git ref (e.g. `refs/heads/main`) |
| `repository.full_name` | string | `owner/repo` format |
| `repository.clone_url` | string | HTTPS clone URL |
| `head_commit.id` | string | Commit SHA |

## Responses

| Status | Condition | Body |
|--------|-----------|------|
| 202 Accepted | Valid push event → Batch job submitted | `{"status": "build_submitted"}` |
| 204 No Content | Non-push event (e.g. `ping`) — ignored | (empty) |
| 400 Bad Request | Invalid or unparseable JSON payload | `{"error": "invalid payload"}` |
| 401 Unauthorized | Missing or invalid `X-Hub-Signature-256` | `{"error": "invalid signature"}` |
| 500 Internal Server Error | Batch API call failed | `{"error": "batch submission failed", "detail": "<error message>"}` |

## Signature Validation

1. Read `WEBHOOK-SECRET` from Azure Key Vault
2. Compute HMAC-SHA256 of raw request body using the secret
3. Compare with `X-Hub-Signature-256` header value (format: `sha256=<hex>`)
4. Use constant-time comparison to prevent timing attacks

## Flow

```
Request received
  → Validate signature (401 if invalid)
  → Check X-GitHub-Event header
    → Not "push" → 204
    → "push" → Parse payload (400 if invalid)
      → Submit Batch job (500 if failed)
      → 202
```
