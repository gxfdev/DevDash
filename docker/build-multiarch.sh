#!/bin/sh
set -e

IMAGE_NAME="${1:-devdash}"
IMAGE_TAG="${2:-latest}"
REGISTRY="${3:-}"

if [ -z "$REGISTRY" ]; then
    FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"
else
    FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
fi

echo "Building multi-arch image: ${FULL_IMAGE}"
echo "Platforms: linux/amd64,linux/arm64,linux/arm/v7"

docker buildx build \
    --platform linux/amd64,linux/arm64,linux/arm/v7 \
    --file docker/Dockerfile.server \
    --tag "${FULL_IMAGE}" \
    --push=false \
    --load=false \
    .

echo ""
echo "Build completed for: ${FULL_IMAGE}"
echo "To push to registry, add --push flag or run:"
echo "  docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 --push -t ${FULL_IMAGE} -f docker/Dockerfile.server ."
