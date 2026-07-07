#!/bin/bash
set -e

echo "=== Electrum Pricing Engine - Deploy to Cloud Run ==="
echo ""

# Configuration - set these before running
PROJECT_ID="${GCP_PROJECT_ID:-electrum-pricing}"
REGION="${GCP_REGION:-asia-southeast1}"
SERVICE_NAME="pricing-engine"
DB_INSTANCE="electrum-pricing-db"
DB_NAME="electrum_pricing"
DB_USER="electrum"
DB_PASS="${DB_PASSWORD:?Set DB_PASSWORD environment variable}"
ADMIN_KEY="${ADMIN_API_KEY:?Set ADMIN_API_KEY}"
READ_KEY="${READ_ONLY_API_KEY:?Set READ_ONLY_API_KEY}"
HMAC_KEY="${HMAC_SECRET:?Set HMAC_SECRET}"

echo "Deploying to project: $PROJECT_ID"
echo "Region: $REGION"
echo ""

# 1. Build and push
echo "[1/3] Building and pushing image..."
gcloud builds submit --tag gcr.io/$PROJECT_ID/$SERVICE_NAME --quiet

# 2. Deploy
echo "[2/3] Deploying to Cloud Run (cold start, min=0)..."
gcloud run deploy $SERVICE_NAME \
  --image gcr.io/$PROJECT_ID/$SERVICE_NAME \
  --region=$REGION \
  --platform=managed \
  --allow-unauthenticated \
  --min-instances=0 \
  --max-instances=100 \
  --concurrency=80 \
  --cpu=1 \
  --memory=512Mi \
  --timeout=60s \
  --add-cloudsql-instances=$PROJECT_ID:$REGION:$DB_INSTANCE \
  --set-env-vars="
ADMIN_API_KEY=$ADMIN_KEY,
READ_ONLY_API_KEY=$READ_KEY,
HMAC_SECRET=$HMAC_KEY,
DATABASE_URL=postgres://$DB_USER:$DB_PASS@/$DB_NAME?host=/cloudsql/$PROJECT_ID:$REGION:$DB_INSTANCE
" --quiet

# 3. Get URL
echo "[3/3] Getting service URL..."
SERVICE_URL=$(gcloud run services describe $SERVICE_NAME \
  --region=$REGION \
  --format="value(status.url)")

echo ""
echo "=== Deployment Complete ==="
echo "URL:        $SERVICE_URL"
echo "Admin Key:  $ADMIN_KEY"
echo "Read Key:   $READ_KEY"
echo "HMAC:       $HMAC_KEY"
echo ""
echo "Quick test:"
echo "curl -H \"X-API-Key: $READ_KEY\" \"$SERVICE_URL/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9\""
