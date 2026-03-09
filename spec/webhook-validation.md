# Webhook Validation Spec

## Signature Verification

### Algorithm
- HMAC-SHA256

### Secret Source
- Azure Key Vault secret: `WEBHOOK-SECRET`
- Set by bootstrap's downloader during initial setup

### Verification Steps

1. Extract `X-Hub-Signature-256` header
   - Missing → 401
   - Format must be `sha256=<hex_string>`
   - Invalid format → 401
2. Read raw request body (before JSON parsing)
3. Compute `HMAC-SHA256(secret, raw_body)`
4. Hex-encode the result
5. Compare with header value using constant-time comparison (`crypto/subtle.ConstantTimeCompare`)
   - Mismatch → 401
   - Match → proceed

### Edge Cases

| Case | Response |
|------|----------|
| Missing `X-Hub-Signature-256` header | 401 |
| Header present but empty value | 401 |
| Header without `sha256=` prefix | 401 |
| Valid format but wrong signature | 401 |
| Empty request body + valid signature for empty body | Proceed (valid) |

## Event Filtering

### Accepted Events

| Event | Action |
|-------|--------|
| `push` | Submit Batch build job |
| `ping` | 204 (webhook registration confirmation) |
| Any other | 204 (ignore) |

### Branch Filtering

- Currently: accept all branches
- Future consideration: configurable branch filter (e.g. `main` only)
