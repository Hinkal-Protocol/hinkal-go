#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

: "${MAVEN_CENTRAL_USERNAME:?MAVEN_CENTRAL_USERNAME not set}"
: "${MAVEN_CENTRAL_PASSWORD:?MAVEN_CENTRAL_PASSWORD not set}"

ARTIFACT_ID="hinkal-android"
SDK_VERSION="$(grep '^sdkVersion=' gradle.properties | cut -d= -f2)"

STAGING_DIR="build/staging-deploy"
ARTIFACT_DIR="${STAGING_DIR}/io/hinkal/${ARTIFACT_ID}/${SDK_VERSION}"
BUNDLE_NAME="${ARTIFACT_ID}-${SDK_VERSION}"
BUNDLE_ZIP="build/${BUNDLE_NAME}-bundle.zip"

echo "==> Building Central bundle..."
if [ ! -d "$ARTIFACT_DIR" ]; then
    echo "error: $ARTIFACT_DIR not found, run './gradlew clean publishReleasePublicationToLocalStagingRepository' first" >&2
    exit 1
fi

rm -f "$BUNDLE_ZIP"
(cd "$STAGING_DIR" && zip -qr "../${BUNDLE_NAME}-bundle.zip" .)

echo "==> Uploading to Central Portal (USER_MANAGED)..."
TOKEN="$(printf '%s:%s' "$MAVEN_CENTRAL_USERNAME" "$MAVEN_CENTRAL_PASSWORD" | base64 | tr -d '\n')"

RESPONSE="$(curl -sS --fail-with-body -X POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -F "bundle=@${BUNDLE_ZIP}" \
    "https://central.sonatype.com/api/v1/publisher/upload?name=${BUNDLE_NAME}&publishingType=USER_MANAGED")"

echo "Deployment ID:"
echo "$RESPONSE"
