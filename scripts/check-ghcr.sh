#!/usr/bin/env bash
set -euo pipefail
REPO="${1:-gxfdev/devdash}"
TAG="${2:-latest}"
TOKEN=$(curl -fsS "https://ghcr.io/token?scope=repository:${REPO}:pull&service=ghcr.io" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
echo "token length: ${#TOKEN}"
curl -sS -o /dev/null -w "manifest HTTP %{http_code}\n" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Accept: application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json" \
  "https://ghcr.io/v2/${REPO}/manifests/${TAG}"
