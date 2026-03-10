# Batch Job Submission Spec

## Overview

Function App creates an ephemeral Batch Pool (autoPool), submits a build job, and returns immediately (fire-and-forget). The Batch node runs the build independently (20-30 min). Pool is auto-deleted when the job completes.

## Job Parameters

Derived from webhook payload + Function App environment:

| Parameter | Source | Description |
|-----------|--------|-------------|
| `repo_url` | Webhook payload (`repository.clone_url`) | Git clone URL |
| `branch` | Webhook payload (`ref`) | Branch ref (e.g. `refs/heads/main`) |
| `commit` | Webhook payload (`head_commit.id`) | Commit SHA |
| `platform` | Function App env (`PLATFORM`) — 미구현: handler가 아직 읽지 않음 | Build target (e.g. WebGL) |
| `image_id` | Function App env (`IMAGE_RESOURCE_ID`) | Full Azure resource ID for specialized VM image |

## Pool Lifecycle (autoPool)

Function creates a Job with `autoPoolSpecification` — Azure Batch automatically creates a dedicated pool for this job and deletes it when the job completes.

| Setting | Value | Reason |
|---------|-------|--------|
| Pool Lifetime | `job` | Pool lives only for this job's duration |
| VM Image | Specialized image from Shared Image Gallery | machine ID preserved → Unity license valid |
| VM Size | Configurable via `.env` → deploy.sh가 Function App setting으로 주입 (e.g. `Standard_D8ads_V5`) | Cost vs build speed tradeoff |
| Node Count | Fixed 1 | Unity 라이선스 정책상 한 계정당 동시 빌드 1개만 허용 |

### Why autoPool (not persistent pool)

- **비용**: Pool이 없으면 VM 비용 0. 빌드할 때만 VM 생성
- **단순성**: Function이 Pool lifecycle을 직접 관리할 필요 없음
- **정리**: Job 완료 시 자동 삭제 — orphaned resource 없음

## Batch Node Execution

Sequence on the Batch node after spin-up:

1. Read Unity license from Key Vault → `/tmp/Unity_lic.ulf`
2. Read `GH_TOKEN` from Key Vault (commit status 보고용)
3. `git clone {repo_url} -b {branch} /project`
4. Execute `scripts/build.sh` (Function App과 함께 배포된 빌드 스크립트):
   - `docker run` game-ci container
   - Mount: `/project`, license file
   - Unity batchmode build
5. Upload build artifacts → Azure Blob Storage (`artifacts/{commit}/build.zip`)
6. Upload build status → Azure Blob Storage (`artifacts/{commit}/status.json`)
7. Report build result → GitHub commit status (PAT via Key Vault)
8. Job completes → autoPool auto-deleted

## Build Result Storage

Batch node uploads results to Azure Blob Storage:

### Artifact: `artifacts/{commit-sha}/build.zip`
- Unity build output

### Status: `artifacts/{commit-sha}/status.json`
```json
{
  "commit": "abc123",
  "status": "success|failure",
  "platform": "WebGL",
  "artifact": "artifacts/abc123/build.zip",
  "duration": 1234,
  "timestamp": "2026-03-09T21:30:00Z"
}
```

## Build Script (`scripts/build.sh`)

빌드 스크립트는 `unity-ci-function` repo의 `scripts/build.sh`에 위치. Function App 배포 시 함께 포함되며, Batch task command로 전달됨.

## Azure Batch API

### Authentication
- Function App uses Managed Identity to access Batch Account
- No explicit credentials needed

### API Calls
1. Create Job with `autoPoolSpecification` (pool created automatically)
2. Add Task to Job with:
   - Command line: `scripts/build.sh` execution
   - Environment variables: `REPO_URL`, `BRANCH`, `COMMIT_SHA`, `PLATFORM`

### Job ID
- Format: `unity-build-{commit-sha}` (idempotent — same commit won't create duplicate jobs)
