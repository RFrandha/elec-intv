# Test Scenario: Full API Walkthrough

Complete test suite for the Dynamic Pricing Engine. Run these commands against a live Cloud Run instance or local Docker setup.

## Prerequisites

1. **Cloud Run instance running:** `https://pricing-engine-599858649457.asia-southeast1.run.app`
   OR
   **Local Docker:** `http://localhost:8080`

2. **API Keys:** Obtain from onboarding PDF or set in environment:
   ```bash
   export READ_ONLY_API_KEY="<your-read-only-key>"
   export ADMIN_API_KEY="<your-admin-key>"
   ```

3. **Base URL:** Set locally or to Cloud Run
   ```bash
   # Local
   export BASE_URL="http://localhost:8080"
   
   # Cloud Run
   export BASE_URL="https://pricing-engine-599858649457.asia-southeast1.run.app"
   ```

## Test 1: Health Check

**Endpoint:** `GET /health`

```bash
curl -s "$BASE_URL/health"
```

**Expected Response:**
```json
{"status":"ok"}
```

---

## Test 2: Basic Pricing Quote

**Endpoint:** `GET /api/v1/pricing`

**Scenario:** Gold tier user (U001, V001), Jakarta Pusat, 0.9 kWh

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9"
```

**Expected Response:**
```json
{
  "quote_id": "<UUID>",
  "vehicle_id": "V001",
  "zone": "jakarta_pusat",
  "kwh_consumed": 0.9,
  "base_rate_per_kwh": 4000,
  "final_price": 3240,
  "valid_until": "<ISO8601-timestamp>",
  "ab_segment": "control"
}
```

**Validation:**
- Status: 200 OK
- `final_price` = 4000 × 0.9 × 0.9 (loyalty discount for gold) = 3240
- `ab_segment` in ["control", "variant"]
- `valid_until` is 30 seconds from now

---

## Test 3: Normal Tier Pricing

**Scenario:** Normal tier user (U002, V002), Jakarta Selatan, 1.5 kWh

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing?vehicle_id=V002&zone=jakarta_selatan&duration_hours=1.5"
```

**Expected Response:**
```json
{
  "quote_id": "<UUID>",
  "vehicle_id": "V002",
  "zone": "jakarta_selatan",
  "kwh_consumed": 1.5,
  "base_rate_per_kwh": 4000,
  "final_price": 6000,
  "valid_until": "<ISO8601-timestamp>",
  "ab_segment": "variant"
}
```

**Validation:**
- `final_price` = 4000 × 1.0 (normal tier) × 1.5 (kWh) = 6000
- Different `ab_segment` likely (deterministic per user_id)

---

## Test 4: Bandung Zone Pricing

**Scenario:** Vehicle in Bandung zone, 0.5 kWh

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing?vehicle_id=V003&zone=bandung&duration_hours=0.5"
```

**Expected Response:**
```json
{
  "quote_id": "<UUID>",
  "vehicle_id": "V003",
  "zone": "bandung",
  "kwh_consumed": 0.5,
  "base_rate_per_kwh": 4000,
  "final_price": 1800,
  "valid_until": "<ISO8601-timestamp>",
  "ab_segment": "variant"
}
```

**Validation:**
- Status: 200 OK
- Bandung zone accepted (not 400 Bad Request)

---

## Test 5: Max kWh (1.8)

**Endpoint:** Edge case — maximum battery capacity

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=1.8"
```

**Expected Response:**
```json
{
  "quote_id": "<UUID>",
  "vehicle_id": "V001",
  "zone": "jakarta_pusat",
  "kwh_consumed": 1.8,
  "base_rate_per_kwh": 4000,
  "final_price": 6480,
  "valid_until": "<ISO8601-timestamp>",
  "ab_segment": "control"
}
```

**Validation:**
- Status: 200 OK (1.8 accepted)
- `final_price` = 4000 × multipliers × 1.8

---

## Test 6: Min kWh (0.1)

**Endpoint:** Minimum allowed

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing?vehicle_id=V005&zone=jakarta_utara&duration_hours=0.1"
```

**Expected Response:**
```json
{
  "quote_id": "<UUID>",
  "vehicle_id": "V005",
  "zone": "jakarta_utara",
  "kwh_consumed": 0.1,
  "base_rate_per_kwh": 4000,
  "final_price": 400,
  "valid_until": "<ISO8601-timestamp>",
  "ab_segment": "variant"
}
```

**Validation:**
- Status: 200 OK
- `final_price` = 4000 × 1.0 × 0.1 = 400

---

## Test 7: Over Max kWh (2.0) — Should Reject

**Endpoint:** Exceeds maximum

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=2.0"
```

**Expected Response:**
```json
{"error":"invalid duration_hours: must be 0.1-1.8"}
```

**Validation:**
- Status: 400 Bad Request
- Error message indicates 1.8 kWh limit

---

## Test 8: Missing Required Parameter

**Endpoint:** Missing `zone`

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing?vehicle_id=V001&duration_hours=0.9"
```

**Expected Response:**
```json
{"error":"missing required parameters"}
```

**Validation:**
- Status: 400 Bad Request

---

## Test 9: Invalid Zone

**Endpoint:** Zone not in whitelist

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing?vehicle_id=V001&zone=mars&duration_hours=0.9"
```

**Expected Response:**
```json
{"error":"invalid zone"}
```

**Validation:**
- Status: 400 Bad Request

---

## Test 10: Missing API Key

**Endpoint:** No authentication

```bash
curl -s "$BASE_URL/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9"
```

**Expected Response:**
```json
{"error":"unauthorized"}
```

**Validation:**
- Status: 401 Unauthorized

---

## Test 11: Pricing Breakdown

**Endpoint:** `GET /api/v1/pricing/{quote_id}/breakdown`

**First:** Get a quote ID

```bash
QUOTE_ID=$(curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9" | \
  grep -o '"quote_id":"[^"]*' | cut -d'"' -f4)
echo $QUOTE_ID
```

**Then:** Get breakdown

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/pricing/$QUOTE_ID/breakdown"
```

**Expected Response:**
```json
{
  "quote_id": "<UUID>",
  "base_rate_per_kwh": 4000,
  "kwh_consumed": 0.9,
  "factors": [
    {
      "name": "demand_multiplier",
      "inputs": {"time": "HH:MM Day"},
      "multiplier": 1.0
    },
    {
      "name": "zone_surge",
      "inputs": {"utilization": 0.96, "zone": "jakarta_pusat"},
      "multiplier": 1.0
    },
    {
      "name": "battery_discount",
      "inputs": {"soc": 30},
      "multiplier": 1.0
    },
    {
      "name": "event_discount",
      "inputs": {"events_active": 0},
      "multiplier": 1.0
    },
    {
      "name": "loyalty_discount",
      "inputs": {"tier": "gold"},
      "multiplier": 0.9
    }
  ],
  "final_price": 3240
}
```

**Validation:**
- 5 factors in order: demand, zone, battery, event, loyalty
- Each factor has name, inputs, multiplier
- Final price matches pricing response

---

## Test 12: Admin — Get AB Tests

**Endpoint:** `GET /api/v1/admin/ab-tests`

```bash
curl -s -H "X-API-Key: $ADMIN_API_KEY" \
  "$BASE_URL/api/v1/admin/ab-tests"
```

**Expected Response:**
```json
[
  {
    "TestID": 1,
    "TestName": "default",
    "SegmentName": "control",
    "ConfigID": 1,
    "IsActive": true
  },
  {
    "TestID": 2,
    "TestName": "default",
    "SegmentName": "variant",
    "ConfigID": 2,
    "IsActive": true
  }
]
```

**Validation:**
- Status: 200 OK
- 2 test configurations (control + variant)
- Each has different ConfigID

---

## Test 13: Admin — Get Config

**Endpoint:** `GET /api/v1/admin/config`

```bash
curl -s -H "X-API-Key: $ADMIN_API_KEY" \
  "$BASE_URL/api/v1/admin/config"
```

**Expected Response:**
```json
{
  "base_price": 4000,
  "demand_rules": [
    {
      "day_of_week": "weekday",
      "hour_start": 5,
      "hour_end": 7,
      "multiplier": 1.3
    },
    {
      "day_of_week": "weekday",
      "hour_start": 0,
      "hour_end": 5,
      "multiplier": 0.9
    }
  ],
  "zone_surge_thresholds": [
    {"min_utilization": 0.8, "multiplier": 1.5},
    {"min_utilization": 0.5, "multiplier": 1.2},
    {"min_utilization": 0, "multiplier": 1.0}
  ],
  "battery_discount_tiers": [
    {"max_soc": 40.0, "multiplier": 0.85},
    {"max_soc": 60.0, "multiplier": 0.95},
    {"max_soc": 100.0, "multiplier": 1.0}
  ]
}
```

**Validation:**
- Status: 200 OK
- Config includes base_price, demand_rules, zone_surge_thresholds, battery_discount_tiers

---

## Test 14: Admin — Fleet State

**Endpoint:** `GET /api/v1/admin/fleet/state`

```bash
curl -s -H "X-API-Key: $ADMIN_API_KEY" \
  "$BASE_URL/api/v1/admin/fleet/state"
```

**Expected Response:**
```json
[
  {
    "zone": "jakarta_pusat",
    "total_vehicles": 100,
    "available_vehicles": 4,
    "utilization_pct": 96,
    "bss_count": 30
  },
  {
    "zone": "jakarta_selatan",
    "total_vehicles": 110,
    "available_vehicles": 5,
    "utilization_pct": 95.45,
    "bss_count": 35
  }
]
```

**Validation:**
- Status: 200 OK
- All 9-10 zones present
- Utilization between 80-100%

---

## Test 15: Admin — Refresh Config

**Endpoint:** `POST /api/v1/admin/config/refresh`

```bash
curl -s -X POST -H "X-API-Key: $ADMIN_API_KEY" \
  -H "Content-Length: 0" \
  "$BASE_URL/api/v1/admin/config/refresh"
```

**Expected Response:**
```json
{"message":"config/events/tiers refreshed"}
```

**Validation:**
- Status: 200 OK
- Indicates refresh successful

---

## Test 16: Admin — AB Test Stats

**Endpoint:** `GET /api/v1/admin/stats/ab-tests`

```bash
curl -s -H "X-API-Key: $ADMIN_API_KEY" \
  "$BASE_URL/api/v1/admin/stats/ab-tests"
```

**Expected Response:**
```json
{
  "control": {
    "requests": 3,
    "avg_price": 3300,
    "total_revenue": 9900
  },
  "variant": {
    "requests": 2,
    "avg_price": 3900,
    "total_revenue": 7800
  }
}
```

**Validation:**
- Status: 200 OK
- Control and variant segments have request counts
- Avg prices and total revenue calculated from audit trail

---

## Test 17: Admin — Read-Only Key Not Authorized

**Endpoint:** Admin endpoint with read-only key

```bash
curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
  "$BASE_URL/api/v1/admin/config"
```

**Expected Response:**
```json
{"error":"unauthorized, admin key required"}
```

**Validation:**
- Status: 401 Unauthorized
- Read-only key rejected for admin endpoints

---

## Rate Limiting Test

**Scenario:** Exceed rate limit on public endpoint (100 req/min)

```bash
# Send 101 requests quickly
for i in {1..101}; do
  curl -s -H "X-API-Key: $READ_ONLY_API_KEY" \
    "$BASE_URL/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9" \
    > /dev/null
done
```

**Expected:** 101st request returns 429 Too Many Requests

```json
{"error":"rate limit exceeded"}
```

---

## Summary

| Test # | Endpoint | Status | Purpose |
|--------|----------|--------|---------|
| 1 | `/health` | 200 | Service alive |
| 2-3 | `/api/v1/pricing` (gold/normal) | 200 | Pricing calculation |
| 4 | `/api/v1/pricing` (bandung) | 200 | Zone validation |
| 5 | `/api/v1/pricing` (1.8 kWh) | 200 | Max kWh accepted |
| 6 | `/api/v1/pricing` (0.1 kWh) | 200 | Min kWh accepted |
| 7 | `/api/v1/pricing` (2.0 kWh) | 400 | Over-max rejection |
| 8 | `/api/v1/pricing` (missing param) | 400 | Validation |
| 9 | `/api/v1/pricing` (bad zone) | 400 | Zone whitelist |
| 10 | `/api/v1/pricing` (no auth) | 401 | Auth required |
| 11 | `/api/v1/pricing/.../breakdown` | 200 | Factor transparency |
| 12 | `/api/v1/admin/ab-tests` | 200 | Admin read |
| 13 | `/api/v1/admin/config` | 200 | Config retrieval |
| 14 | `/api/v1/admin/fleet/state` | 200 | Fleet state |
| 15 | `/api/v1/admin/config/refresh` | 200 | Manual refresh |
| 16 | `/api/v1/admin/stats/ab-tests` | 200 | Analytics |
| 17 | `/api/v1/admin/*` (read key) | 401 | Auth enforcement |
| Rate | Public endpoint (101x) | 429 | Rate limiting |

**All 17+ tests passing = full API operational.**
