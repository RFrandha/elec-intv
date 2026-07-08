# API Documentation

## Overview

The Dynamic Pricing Engine calculates battery swap pricing for Electrum's EV fleet using composable factors: demand (time-of-day), zone surge (fleet utilization), battery discount (return SoC), events (promotions), and loyalty (tier). All factors are multipliers applied sequentially.

### Base URL

| Environment | URL |
|-------------|-----|
| **Cloud Run (production)** | `https://pricing-engine-599858649457.asia-southeast1.run.app` |
| **Local (Docker Compose)** | `http://localhost:8080` |

### Content Type

All requests and responses use `application/json`.

### Authentication

Two-tier API key system. Keys passed via `X-API-Key` header.

| Tier | Key Pattern | Accessible Endpoints | Rate Limit |
|------|-------------|---------------------|------------|
| **Read-only** | `<READ_ONLY_API_KEY>` | `/health`, `/api/v1/pricing/*` | 100 req/min |
| **Admin** | `<ADMIN_API_KEY>` | All endpoints including `/api/v1/admin/*` | 10 req/min |

**Obtaining keys:** Provided separately in onboarding PDF (not in repository).

---

## Common Responses

### Success

```json
{
  "quote_id": "550e8400-e29b-41d4-a716-446655440000",
  "final_price": 3240
}
```

### Error

```json
{
  "error": "description of what went wrong"
}
```

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| `200 OK` | Request succeeded |
| `201 Created` | Resource created (events) |
| `400 Bad Request` | Invalid input (missing params, bad zone, out of bounds) |
| `401 Unauthorized` | Missing or invalid API key |
| `404 Not Found` | Resource not found (vehicle, quote, event) |
| `429 Too Many Requests` | Rate limit exceeded |
| `500 Internal Server Error` | Server-side failure |

---

## Rate Limiting

- **Public endpoints:** 100 requests per minute per IP
- **Admin endpoints:** 10 requests per minute per key
- Rate limit applies independently per endpoint group
- When exceeded:

```json
{"error":"rate limit exceeded"}
```

**Note:** POST requests to admin endpoints require `Content-Length: 0` header to count against rate limit correctly on Cloud Run.

```
-H "Content-Length: 0"
```

---

## Endpoints

---

### 1. Health Check

`GET /health`

Verify the service is running.

**Example Request:**
```bash
curl -s "https://<host>/health"
```

**Example Response (200):**
```json
{"status":"ok"}
```

**Errors:** None (no auth required).

---

### 2. Get Pricing Quote

`GET /api/v1/pricing`

Calculate price for a battery swap. Requires read-only or admin API key.

**Query Parameters:**

| Parameter | Type | Required | Description | Constraints |
|-----------|------|----------|-------------|-------------|
| `vehicle_id` | string | ✅ | Vehicle identifier | Must exist in system (V001-V007 seeded) |
| `zone` | string | ✅ | Geographic zone | Must be one of 10 allowed zones |
| `duration_hours` | float | ✅ | kWh energy consumed | 0.1 - 1.8 (Electrum max battery) |

**Notes:**
- `duration_hours` represents **kWh energy consumed**, not time duration. Parameter name preserved per test specification.
- Price quote valid for **30 seconds** (TTL returned in `valid_until`).
- A/B segment assigned deterministically: `SHA256(user_id) % 100`.
- All 6 multipliers applied: demand, zone surge, battery, event, loyalty, kWh.

**Allowed Zones:**
```
jakarta_pusat, jakarta_selatan, jakarta_barat,
jakarta_timur, jakarta_utara, bogor,
depok, tangerang, bekasi, bandung
```

**Example Request (gold subscriber, 0.9 kWh, Jakarta Pusat):**
```bash
curl -s -H "X-API-Key: <READ_ONLY_API_KEY>" \
  "https://<host>/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9"
```

**Example Response (200):**
```json
{
  "quote_id": "2f3b312f-697d-4b0c-a5aa-50a26e0d4add",
  "vehicle_id": "V001",
  "zone": "jakarta_pusat",
  "kwh_consumed": 0.9,
  "base_rate_per_kwh": 4000,
  "final_price": 3240,
  "valid_until": "2026-07-08T05:53:25Z",
  "ab_segment": "control"
}
```

**Example Request (normal tier, 1.5 kWh, Jakarta Selatan):**
```bash
curl -s -H "X-API-Key: <READ_ONLY_API_KEY>" \
  "https://<host>/api/v1/pricing?vehicle_id=V002&zone=jakarta_selatan&duration_hours=1.5"
```

**Example Response (200):**
```json
{
  "quote_id": "824e8329-cfb5-477c-8dc1-e25bf0d574af",
  "vehicle_id": "V002",
  "zone": "jakarta_selatan",
  "kwh_consumed": 1.5,
  "base_rate_per_kwh": 4000,
  "final_price": 6000,
  "valid_until": "2026-07-08T05:53:26Z",
  "ab_segment": "variant"
}
```

**Error Responses:**

| Status | Condition | Example |
|--------|-----------|---------|
| 400 | Missing parameter | `{"error":"missing required parameters"}` |
| 400 | Invalid zone | `{"error":"invalid zone"}` |
| 400 | kWh out of bounds | `{"error":"invalid duration_hours: must be 0.1-1.8"}` |
| 400 | Vehicle not found | `{"error":"vehicle not found"}` |
| 401 | No API key | `{"error":"unauthorized"}` |
| 401 | Invalid API key | `{"error":"invalid API key"}` |
| 429 | Rate limit | `{"error":"rate limit exceeded"}` |

---

### 3. Get Pricing Breakdown

`GET /api/v1/pricing/:quote_id/breakdown`

Retrieve detailed factor breakdown for a previously calculated quote. Requires read-only or admin API key.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `quote_id` | UUID | Quote ID returned from pricing endpoint |

**Notes:**
- Quote breakdown available for **30 seconds** after generation (in-memory cache).
- Each factor shows its `name`, `inputs`, and `multiplier` for full transparency.

**Example Request:**
```bash
QUOTE_ID=$(curl -s -H "X-API-Key: <READ_ONLY_API_KEY>" \
  "https://<host>/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9" \
  | grep -o '"quote_id":"[^"]*' | cut -d'"' -f4)
curl -s -H "X-API-Key: <READ_ONLY_API_KEY>" \
  "https://<host>/api/v1/pricing/$QUOTE_ID/breakdown"
```

**Example Response (200):**
```json
{
  "quote_id": "2f3b312f-697d-4b0c-a5aa-50a26e0d4add",
  "base_rate_per_kwh": 4000,
  "kwh_consumed": 0.9,
  "factors": [
    {
      "name": "demand_multiplier",
      "inputs": {"time": "05:47 Wed"},
      "multiplier": 1.0
    },
    {
      "name": "zone_surge",
      "inputs": {"utilization": 0.84, "zone": "jakarta_pusat"},
      "multiplier": 1.0
    },
    {
      "name": "battery_discount",
      "inputs": {"kwh_consumed": 0.9, "return_soc": 50},
      "multiplier": 0.90
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

**Error Responses:**

| Status | Condition | Example |
|--------|-----------|---------|
| 404 | Quote not found / expired | `{"error":"quote not found"}` |

---

### 4. Get Active Config

`GET /api/v1/admin/config`

Retrieve the currently active pricing configuration. Requires admin API key.

**Example Request:**
```bash
curl -s -H "X-API-Key: <ADMIN_API_KEY>" \
  "https://<host>/api/v1/admin/config"
```

**Example Response (200):**
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
    {"min_soc": 60.0, "multiplier": 0.80},
    {"min_soc": 40.0, "multiplier": 0.90},
    {"min_soc": 0.0, "multiplier": 1.0}
  ]
}
```

---

### 5. Update Config

`PUT /api/v1/admin/config`

Create a new version of pricing configuration. Does NOT take effect until cache is refreshed (see #6). Requires admin API key.

**Request Body Schema:**

```json
{
  "base_price": 4500,
  "demand_rules": [
    {
      "day_of_week": "weekday|weekend",
      "hour_start": 0,
      "hour_end": 23,
      "multiplier": 1.0
    }
  ],
  "zone_surge_thresholds": [
    {
      "min_utilization": 0.0,
      "multiplier": 1.0
    }
  ],
  "battery_discount_tiers": [
    {
      "min_soc": 0.0,
      "multiplier": 1.0
    }
  ]
}
```

**Validation Rules:**

| Field | Rule |
|-------|------|
| `base_price` | Must be > 0 and ≤ 100,000 |
| `demand_rules` | `hour_start` < `hour_end`, multiplier > 0 |
| `zone_surge_thresholds` | `min_utilization` 0-1, multiplier > 0 |
| `battery_discount_tiers` | `min_soc` 0-100, multiplier > 0 |

**Example Request:**
```bash
curl -s -X PUT -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "base_price": 4500,
    "demand_rules": [
      {"day_of_week": "weekday", "hour_start": 5, "hour_end": 7, "multiplier": 1.3},
      {"day_of_week": "weekday", "hour_start": 0, "hour_end": 5, "multiplier": 0.9}
    ],
    "zone_surge_thresholds": [
      {"min_utilization": 0.8, "multiplier": 1.5},
      {"min_utilization": 0.5, "multiplier": 1.2},
      {"min_utilization": 0, "multiplier": 1.0}
    ],
    "battery_discount_tiers": [
      {"min_soc": 60.0, "multiplier": 0.80},
      {"min_soc": 40.0, "multiplier": 0.90},
      {"min_soc": 0.0, "multiplier": 1.0}
    ]
  }' \
  "https://<host>/api/v1/admin/config"
```

**Example Response (200):**
```json
{"message":"config updated","version":3}
```

**Error Responses:**

| Status | Condition | Example |
|--------|-----------|---------|
| 400 | Invalid config | `{"error":"base_price must be positive and < 100000"}` |

**Config Versioning:**
- Each update creates a new row in `pricing_configs` with auto-incrementing `version`.
- Previous versions are preserved for rollback.
- View history via `GET /api/v1/admin/config/history`.
- **New config does NOT take effect until cache is manually refreshed.**

---

### 6. Refresh Cache

`POST /api/v1/admin/config/refresh`

Reload pricing config, events, tiers, and AB test configs from database into in-memory cache. Required after config update. Requires admin API key.

**Notes:**
- Refreshes: pricing config, active events, subscription tiers, AB test configs
- Must send `Content-Length: 0` header

**Example Request:**
```bash
curl -s -X POST -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Length: 0" \
  "https://<host>/api/v1/admin/config/refresh"
```

**Example Response (200):**
```json
{"message":"config/events/tiers refreshed"}
```

---

### 7. Get Config History

`GET /api/v1/admin/config/history`

View all versions of pricing configuration. Requires admin API key.

**Example Request:**
```bash
curl -s -H "X-API-Key: <ADMIN_API_KEY>" \
  "https://<host>/api/v1/admin/config/history"
```

**Example Response (200):**
```json
[
  {
    "config_id": 1,
    "version": 1,
    "created_at": "2026-07-07T15:01:50Z",
    "created_by": "system"
  },
  {
    "config_id": 12,
    "version": 2,
    "created_at": "2026-07-08T04:46:46Z",
    "created_by": "system"
  }
]
```

---

### 8. Get Fleet State

`GET /api/v1/admin/fleet/state`

View current fleet utilization across all zones. Requires admin API key.

**Example Request:**
```bash
curl -s -H "X-API-Key: <ADMIN_API_KEY>" \
  "https://<host>/api/v1/admin/fleet/state"
```

**Example Response (200):**
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
    "zone": "bandung",
    "total_vehicles": 60,
    "available_vehicles": 18,
    "utilization_pct": 70,
    "bss_count": 12
  }
]
```

**Fields:**
| Field | Description |
|-------|-------------|
| `zone` | Geographic zone |
| `total_vehicles` | Total vehicles registered in zone |
| `available_vehicles` | Vehicles currently not rented |
| `utilization_pct` | Calculated: `(1 - available/total) × 100` |
| `bss_count` | Number of Battery Swap Stations in zone |

---

### 9. Refresh Fleet

`POST /api/v1/admin/fleet/refresh`

Recalculate fleet state from current vehicle data. Simulates realistic utilization with time-of-day variation. Requires admin API key.

**Example Request:**
```bash
curl -s -X POST -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Length: 0" \
  "https://<host>/api/v1/admin/fleet/refresh"
```

**Example Response (200):**
```json
{"message":"fleet state refreshed"}
```

---

### 10. List Events

`GET /api/v1/admin/events`

List all promotional events, ordered by most recent. Requires admin API key.

**Example Request:**
```bash
curl -s -H "X-API-Key: <ADMIN_API_KEY>" \
  "https://<host>/api/v1/admin/events"
```

**Example Response (200):**
```json
[
  {
    "id": 1,
    "name": "Ramadan Special",
    "zone": "jakarta_pusat",
    "bss_id": null,
    "start_time": "2026-07-08T00:00:00Z",
    "end_time": "2026-12-31T23:59:59Z",
    "discount_multiplier": 0.8,
    "is_active": true
  }
]
```

**Fields:**
| Field | Description |
|-------|-------------|
| `zone` | Zone-scoped event (null = nationwide) |
| `bss_id` | BSS-scoped event (null = zone-wide or nationwide) |
| `discount_multiplier` | Applied to pricing (0.8 = 20% off) |
| `is_active` | Whether current time falls within start/end window |

---

### 11. Create Event

`POST /api/v1/admin/events`

Create a new promotional event. Takes effect after cache refresh. Requires admin API key.

**Request Body:**

```json
{
  "name": "Ramadan Special",
  "zone": "jakarta_pusat",
  "start_time": "2026-07-08T00:00:00Z",
  "end_time": "2026-12-31T23:59:59Z",
  "discount_multiplier": 0.8
}
```

**Fields:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | ✅ | Event name |
| `zone` | string | ❌ | Zone scope (omit for nationwide) |
| `bss_id` | int | ❌ | BSS scope (omit for zone/nationwide) |
| `start_time` | ISO8601 | ✅ | Event start |
| `end_time` | ISO8601 | ✅ | Event end |
| `discount_multiplier` | float | ✅ | 0.01-2.0 |

**Example Request:**
```bash
curl -s -X POST -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Ramadan Special",
    "zone": "jakarta_pusat",
    "start_time": "2026-07-08T00:00:00Z",
    "end_time": "2026-12-31T23:59:59Z",
    "discount_multiplier": 0.8
  }' \
  "https://<host>/api/v1/admin/events"
```

**Notes:**
- After creating, refresh cache via `POST /api/v1/admin/config/refresh` for the event to take effect in pricing.

**Example Response (201):**
```json
{"event_id": 1, "message": "event created"}
```

**Error Responses:**

| Status | Condition | Example |
|--------|-----------|---------|
| 400 | Invalid body | `{"error":"invalid request body"}` |
| 400 | Invalid multiplier | `{"error":"discount_multiplier must be 0.01-2.0"}` |

---

### 12. Delete Event

`DELETE /api/v1/admin/events/:id`

Cancel a promotional event. Requires admin API key.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | int | Event ID from list |

**Example Request:**
```bash
curl -s -X DELETE -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Length: 0" \
  "https://<host>/api/v1/admin/events/1"
```

**Example Response (200):**
```json
{"message":"event deleted"}
```

---

### 13. List AB Tests

`GET /api/v1/admin/ab-tests`

List current A/B test segment-to-config mappings. Requires admin API key.

**Example Request:**
```bash
curl -s -H "X-API-Key: <ADMIN_API_KEY>" \
  "https://<host>/api/v1/admin/ab-tests"
```

**Example Response (200):**
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

**Notes:**
- A/B testing uses **config routing** (different config version per segment), not a single multiplier.
- Segment assignment: `SHA256(user_id) % 100` — deterministic.
- Each segment loads a full pricing configuration (base price, demand rules, etc.).

---

### 14. Delete AB Test

`DELETE /api/v1/admin/ab-tests/:id`

Remove an A/B test segment mapping. Requires admin API key.

**Example Request:**
```bash
curl -s -X DELETE -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Length: 0" \
  "https://<host>/api/v1/admin/ab-tests/2"
```

**Example Response (200):**
```json
{"message":"AB test config deleted"}
```

---

### 15. AB Test Statistics

`GET /api/v1/admin/stats/ab-tests`

View A/B test results from audit trail. Requires admin API key.

**Example Request:**
```bash
curl -s -H "X-API-Key: <ADMIN_API_KEY>" \
  "https://<host>/api/v1/admin/stats/ab-tests"
```

**Example Response (200):**
```json
{
  "control": {
    "requests": 3,
    "avg_price": 4440,
    "total_revenue": 13320
  },
  "variant": {
    "requests": 3,
    "avg_price": 2733.33,
    "total_revenue": 8200
  }
}
```

**Fields:**
| Field | Description |
|-------|-------------|
| `requests` | Number of pricing calls in this segment |
| `avg_price` | Average final price across all requests |
| `total_revenue` | Sum of all final prices |

**Notes:**
- Data sourced from `pricing_audit` table (append-only, HMAC-signed).
- Zeros if no pricing requests have been made.

---

### 16. Pricing Statistics

`GET /api/v1/admin/stats/pricing`

View pricing statistics by zone from audit trail. Requires admin API key.

**Example Request:**
```bash
curl -s -H "X-API-Key: <ADMIN_API_KEY>" \
  "https://<host>/api/v1/admin/stats/pricing"
```

**Example Response (200):**
```json
[
  {
    "zone": "jakarta_pusat",
    "avg_price": 3240,
    "count": 5
  },
  {
    "zone": "jakarta_selatan",
    "avg_price": 6000,
    "count": 2
  }
]
```

**Notes:**
- Returns time-bucketed averages from audit trail.
- Includes only data within configured time range.

---

## Appendices

### A. Pricing Formula

```
final_price = base_rate
            × demand_multiplier(time_of_day, day_of_week)
            × zone_surge_factor(zone_utilization)
            × battery_discount_factor(return_soc)
            × event_discount(zone, time)
            × loyalty_discount(subscription_tier)
            × kwh_consumed
```

For detailed explanation, see [Pricing Calculation Walkthrough](UBIQUITOUS_LANGUAGE.md#pricing-calculation-walkthrough).

### B. Available Zones (10 total)

```
jakarta_pusat       jakarta_selatan     jakarta_barat
jakarta_timur       jakarta_utara       bogor
depok               tangerang           bekasi
bandung
```

### C. Factor Reference

| Factor | Input | Thresholds | Purpose |
|--------|-------|------------|---------|
| Demand | hour, day_of_week | 5-7AM weekday=1.3×, 0-5AM=0.9× | Balance time-of-day load |
| Zone Surge | zone_utilization | >80%=1.5×, 50-80%=1.2×, <50%=1.0× | Balance geographic demand |
| Battery Discount | kWh consumed | return SoC ≥60%=0.80×, ≥40%=0.90×, <40%=1.0× | Reward returning chargeable batteries |
| Event | zone (optional) | Configurable per event (e.g., 0.8×) | Marketing promotions |
| Loyalty | subscription_tier | Gold=0.9×, Normal=1.0× | Reward subscribers |
| kWh | duration_hours | 0.1-1.8 | Pay for actual energy |

### D. Configuration Schema

```json
{
  "base_price": 4000,
  "demand_rules": [
    {
      "day_of_week": "weekday|weekend",
      "hour_start": 0,
      "hour_end": 23,
      "multiplier": 1.0
    }
  ],
  "zone_surge_thresholds": [
    {
      "min_utilization": 0.0,
      "multiplier": 1.0
    }
  ],
  "battery_discount_tiers": [
    {
      "min_soc": 0.0,
      "multiplier": 1.0
    }
  ]
}
```

### E. Quick Reference: Common curl Flags

| Flag | Purpose |
|------|---------|
| `-s` | Silent (no progress output) |
| `-H "X-API-Key: <key>"` | Authentication |
| `-H "Content-Type: application/json"` | JSON request body |
| `-H "Content-Length: 0"` | Required for POST requests on Cloud Run |
| `-X POST` | HTTP method override |
| `-d '{"key":"value"}'` | Request body |
| `-d "@file.json"` | Request body from file |
