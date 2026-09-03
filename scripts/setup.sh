#!/usr/bin/env bash
# One-time setup: creates an isolated 'devfest' gcloud configuration for
# your PERSONAL Google account. Your work gcloud config is never touched:
# every command is scoped with CLOUDSDK_ACTIVE_CONFIG_NAME=devfest and we
# never switch the active configuration.
set -euo pipefail

CONFIG_NAME="devfest"
cd "$(dirname "$0")/.."

bold() { printf "\n\033[1m%s\033[0m\n" "$*"; }
ask()  { read -r -p "$1 " REPLY_VALUE; }

command -v gcloud >/dev/null || { echo "gcloud is not installed. Install the Google Cloud SDK first."; exit 1; }

bold "1/6 · gcloud configuration '$CONFIG_NAME' (isolated from your work setup)"
if gcloud config configurations describe "$CONFIG_NAME" >/dev/null 2>&1; then
  echo "Configuration '$CONFIG_NAME' already exists — reusing it."
else
  gcloud config configurations create "$CONFIG_NAME" --no-activate
  echo "Created configuration '$CONFIG_NAME' (your active config is unchanged)."
fi

bold "2/6 · Log in with your PERSONAL Google account"
echo "A browser window will open. Pick your personal account, NOT your work one."
ask "Press Enter to continue…"
CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud auth login

ACCOUNT=$(CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud config get-value account 2>/dev/null)
echo "Logged in as: $ACCOUNT"

bold "3/6 · Choose or create a GCP project"
echo "Your projects on that account:"
CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud projects list --format="table(projectId,name)" 2>/dev/null || true
ask "Project ID to use (or a new ID to create, e.g. rapbattle-devfest):"
PROJECT_ID="$REPLY_VALUE"
if ! CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud projects describe "$PROJECT_ID" >/dev/null 2>&1; then
  echo "Creating project '$PROJECT_ID'…"
  CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud projects create "$PROJECT_ID"
fi
CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud config set project "$PROJECT_ID"

bold "4/6 · Billing"
if ! CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud billing projects describe "$PROJECT_ID" --format="value(billingEnabled)" 2>/dev/null | grep -q True; then
  echo "Billing is NOT enabled on '$PROJECT_ID'. Cloud Build/Run need it (free tier covers this demo)."
  echo "Enable it here, then re-run this script if the next step fails:"
  echo "  https://console.cloud.google.com/billing/linkedaccount?project=$PROJECT_ID"
  ask "Press Enter once billing is linked (or to try anyway)…"
fi

bold "5/6 · Enable required APIs (Cloud Run + Cloud Build + Artifact Registry)"
CLOUDSDK_ACTIVE_CONFIG_NAME=$CONFIG_NAME gcloud services enable \
  run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com

bold "6/6 · Local .env"
if [ ! -f .env ]; then cp .env.example .env; fi
grep -q "^GCP_PROJECT=$PROJECT_ID$" .env 2>/dev/null || \
  sed -i '' -e "s/^GCP_PROJECT=.*/GCP_PROJECT=$PROJECT_ID/" .env
if ! grep -q "^GEMINI_API_KEY=.\+" .env; then
  echo "Get a Gemini API key with your PERSONAL account: https://aistudio.google.com/apikey"
  ask "Paste your GEMINI_API_KEY:"
  sed -i '' -e "s|^GEMINI_API_KEY=.*|GEMINI_API_KEY=$REPLY_VALUE|" .env
fi
if ! grep -q "^PRESENTER_TOKEN=.\+" .env; then
  TOKEN=$(openssl rand -hex 8)
  sed -i '' -e "s/^PRESENTER_TOKEN=.*/PRESENTER_TOKEN=$TOKEN/" .env
  echo "Generated PRESENTER_TOKEN=$TOKEN (stored in .env)"
fi

bold "Done!"
echo "  • Deploy any time with:  ./scripts/deploy.sh   (or: make deploy)"
echo "  • Your work gcloud config was never modified."
