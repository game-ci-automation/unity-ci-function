#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load .env
ENV_FILE="$PROJECT_ROOT/.env"
if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: .env not found. Copy .env.example to .env and fill in your values."
  exit 1
fi
source "$ENV_FILE"

: "${RESOURCE_GROUP_NAME:?RESOURCE_GROUP_NAME is not set in .env}"
: "${FUNCTION_APP_NAME:?FUNCTION_APP_NAME is not set in .env}"

cd "$PROJECT_ROOT"

echo "=== Building Go binary (linux/amd64) ==="
GOOS=linux GOARCH=amd64 go build -o handler cmd/handler/main.go

echo "=== Creating deployment package ==="
py -c "
import zipfile, os
with zipfile.ZipFile('deploy.zip', 'w', zipfile.ZIP_DEFLATED) as zf:
    zf.write('handler')
    zf.write('host.json')
    for root, dirs, files in os.walk('github-webhook'):
        for f in files:
            path = os.path.join(root, f).replace(os.sep, '/')
            zf.write(os.path.join(root, f), path)
"

echo "=== Deploying to $FUNCTION_APP_NAME ==="
az functionapp deployment source config-zip \
  --resource-group "$RESOURCE_GROUP_NAME" \
  --name "$FUNCTION_APP_NAME" \
  --src deploy.zip

echo "=== Cleanup ==="
rm -f handler deploy.zip

echo "=== Done ==="
