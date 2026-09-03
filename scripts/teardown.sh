#!/usr/bin/env bash
# Delete the Cloud Run service after the event so nothing keeps billing.
set -euo pipefail

CONFIG_NAME="devfest"
SERVICE="rapbattle"
cd "$(dirname "$0")/.."

set -a; source .env; set +a
REGION="${GCP_REGION:-us-central1}"

CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud run services delete "$SERVICE" \
  --project "$GCP_PROJECT" --region "$REGION" --quiet
echo "Service deleted. Sleep well."
