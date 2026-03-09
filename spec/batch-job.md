# Batch Job Submission Spec

## Overview

Function App submits a build job to Azure Batch and returns immediately (fire-and-forget). The Batch node runs the build independently (20-30 min).

## Job Parameters

Derived from webhook payload + Function App environment:

| Parameter | Source | Description |
|-----------|--------|-------------|
| `repo_url` | Webhook payload (`repository.clone_url`) | Git clone URL |
| `branch` | Webhook payload (`ref`) | Branch ref (e.g. `refs/heads/main`) |
| `commit` | Webhook payload (`head_commit.id`) | Commit SHA |
| `platform` | Function App env (`PLATFORM`) | Build target (e.g. WebGL) |
| `image_id` | Function App env (`IMAGE_GALLERY_NAME`, `IMAGE_DEFINITION_NAME`) | Specialized VM image |

## Batch Pool Configuration

| Setting | Value | Reason |
|---------|-------|--------|
| VM Image | Specialized image from Shared Image Gallery | machine ID preserved → Unity license valid |
| VM Size | TBD | Cost vs build speed tradeoff |
| Node Count | Fixed 1 | Unity 라이선스 정책상 한 계정당 동시 빌드 1개만 허용 |

## Batch Node Execution

Sequence on the Batch node after spin-up:

1. Read Unity license from Key Vault → `/tmp/Unity_lic.ulf`
2. Read `GH_TOKEN` from Key Vault (commit status 보고용)
3. `git clone {repo_url} -b {branch} /project`
4. Execute `scripts/build.sh` (Function App과 함께 배포된 빌드 스크립트):
   - `docker run` game-ci container
   - Mount: `/project`, license file
   - Unity batchmode build
5. Upload build artifacts → Azure Blob Storage (`{branch}/{commit}/`)
6. Report build result → GitHub commit status (PAT via Key Vault)
7. Node auto-deleted after completion

## Build Script (`scripts/build.sh`)

빌드 스크립트는 `unity-ci-function` repo의 `scripts/build.sh`에 위치. Function App 배포 시 함께 포함되며, Batch task command로 전달됨.

## Azure Batch API

### Authentication
- Function App uses Managed Identity to access Batch Account
- No explicit credentials needed

### API Calls
1. Create Job (or reuse existing pool job)
2. Add Task to Job with:
   - Command line: `scripts/build.sh` execution
   - Environment variables: repo_url, branch, commit, platform
   - Resource files: license from Key Vault
