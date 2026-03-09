# Error Handling Spec

## HTTP Response Strategy

Function App is a thin trigger — validate, submit, return. No retries within the Function itself.

| Error Type | HTTP Status | Action |
|------------|-------------|--------|
| Invalid signature | 401 | Reject immediately, log warning |
| Invalid JSON body | 400 | Reject immediately, log warning |
| Non-push event | 204 | Ignore silently |
| Pool creation fails | 500 | Return error, log error |
| Batch API unreachable | 500 | Return error, log error |
| Batch API rejects job | 500 | Return error, log error |
| Key Vault unreachable | 500 | Return error, log error |

## Logging

| Level | When |
|-------|------|
| INFO | Valid push received, Batch job submitted |
| WARN | Invalid signature attempt, malformed payload |
| ERROR | Batch API failure, Key Vault failure |

## Key Vault Access Failure

- `WEBHOOK-SECRET` read fails → cannot validate any request → 500 for all incoming
- `GH_TOKEN` read fails → Batch node cannot report commit status (build still runs, status not reported)
- These are critical failures — should be monitored

## Pool Creation Failure

- autoPool creation fails → Job creation fails → 500
- GitHub will retry webhook delivery (up to 3 times by default)
- Retry is safe — autoPool with same job ID is idempotent

## Batch Submission Failure

- Function returns 500 to GitHub
- GitHub will retry webhook delivery (up to 3 times by default)
- Retry is acceptable since Batch job submission is idempotent (same commit won't produce duplicate builds if job ID includes commit SHA)

## Build Queue

- autoPool creates 1 node per job (Unity license restriction)
- 동시 push가 들어오면 각각 별도 Job+Pool이 생성됨 (단, Unity 라이선스 제한으로 실질적 동시 빌드는 불가)
- Function은 항상 202 반환 — queue 상태와 무관

## Timeout

- Azure Functions Consumption Plan: 5 min max execution time
- Expected execution: < 5 seconds (validate + submit)
- If Batch API is slow: Function may timeout → GitHub retries
