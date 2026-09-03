#!/usr/bin/env bash
# One-command deploy to Cloud Run on your PERSONAL account.
# Uses the isolated 'devfest' gcloud configuration created by setup.sh;
# your work gcloud config is untouched.
set -euo pipefail

CONFIG_NAME="devfest"
SERVICE="rapbattle"
cd "$(dirname "$0")/.."

[ -f .env ] || { echo "No .env found. Run ./scripts/setup.sh first."; exit 1; }
set -a; source .env; set +a
: "${GCP_PROJECT:?GCP_PROJECT missing in .env — run setup.sh}"
: "${GEMINI_API_KEY:?GEMINI_API_KEY missing in .env — run setup.sh}"
: "${PRESENTER_TOKEN:?PRESENTER_TOKEN missing in .env — run setup.sh}"
REGION="${GCP_REGION:-us-central1}"

echo "Deploying '$SERVICE' to project '$GCP_PROJECT' ($REGION)…"

# --max-instances=1 is load-bearing: battle state lives in memory, so a
# single instance must own the whole show. --session-affinity and a long
# --timeout keep WebSockets happy.
CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud run deploy "$SERVICE" \
  --source . \
  --project "$GCP_PROJECT" \
  --region "$REGION" \
  --allow-unauthenticated \
  --max-instances 1 \
  --min-instances 1 \
  --concurrency 300 \
  --timeout 3600 \
  --memory 512Mi \
  --session-affinity \
  --set-env-vars "GEMINI_API_KEY=$GEMINI_API_KEY,PRESENTER_TOKEN=$PRESENTER_TOKEN"

URL=$(CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud run services describe "$SERVICE" \
  --project "$GCP_PROJECT" --region "$REGION" --format="value(status.url)")

echo ""
echo "🎤 Show is live!"
echo "   Audience: $URL"
echo "   Stage:    $URL/stage?token=$PRESENTER_TOKEN"
echo ""
echo "Reminder: '--min-instances 1' keeps one instance warm (costs ~cents/hour)."
echo "After the event, tear down with: ./scripts/teardown.sh"
