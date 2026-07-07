# Dynamic Pricing Engine for Electrum EV Battery Swaps

Electrum Take Home Test — Case 2: Dynamic Pricing Engine for EV Rentals

## Deployed Instance

Base URL: `https://pricing-engine-599858649457.asia-southeast1.run.app`

API keys are provided separately in the onboarding PDF (not in this repository).

### Quick Test

```bash
# Replace <READONLY_KEY> with key from the onboarding PDF
curl -H "X-API-Key: <READONLY_KEY>" \
  "https://pricing-engine-599858649457.asia-southeast1.run.app/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9"
```

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.21+ (for local development)

### Run Locally

```bash
docker compose up --build
```

API runs at `http://localhost:8080`

### Run Tests

```bash
go test -v ./src/tests/
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  HTTP API Layer (Go net/http)                           │
│  ├─ GET /api/v1/pricing (public)                        │
│  ├─ GET /api/v1/pricing/{quote_id}/breakdown           │
│  └─ Admin endpoints (config, events, fleet, analytics) │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│  Pricing Engine (internal/pricing)                       │
│  ├─ Factor chain (demand, zone, battery, event, loyalty)│
│  ├─ Config hot-reload (30s polling)                     │
│  ├─ HMAC audit trail (tamper-evident)                   │
│  └─ A/B testing (config routing)                        │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│  PostgreSQL (Cloud SQL)                                  │
│  ├─ pricing.vehicles (fleet state)                      │
│  ├─ pricing.pricing_configs (versioned JSONB)           │
│  ├─ pricing.pricing_audit (append-only, HMAC-signed)    │
│  ├─ pricing.events (promotions)                         │
│  ├─ pricing.tiers (loyalty tiers)                       │
│  └─ pricing.users (subscribers)                         │
└─────────────────────────────────────────────────────────┘
```

## API Overview

### Pricing Endpoint

**Request:**
```bash
curl -H "X-API-Key: demo-read-only-1234" \
  "http://localhost:8080/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9"
```

**Response:**
```json
{
  "quote_id": "Q-a1b2c3d4",
  "vehicle_id": "V001",
  "zone": "jakarta_pusat",
  "kwh_consumed": 0.9,
  "base_rate_per_kwh": 4000,
  "final_price": 4977,
  "valid_until": "2026-07-07T10:59:15Z",
  "ab_segment": "control"
}
```

**Parameters:**
- `vehicle_id` (required): Vehicle identifier
- `zone` (required): Geographic zone (jakarta_pusat, jakarta_selatan, etc.)
- `duration_hours` (required): kWh energy consumed (0.1-1.8, max battery capacity)
- NOTE: `duration_hours` parameter name kept for test compliance; represents kWh energy

### Pricing Breakdown Endpoint

```bash
curl -H "X-API-Key: demo-read-only-1234" \
  "http://localhost:8080/api/v1/pricing/Q-a1b2c3d4/breakdown"
```

Shows each factor's contribution to final price.

### Admin Endpoints

**Get Current Config:**
```bash
curl -H "X-API-Key: admin-secure-key-5678" \
  "http://localhost:8080/api/v1/admin/config"
```

**Update Config:**
```bash
curl -X PUT -H "X-API-Key: admin-secure-key-5678" \
  -H "Content-Type: application/json" \
  -d '{
    "base_price": 4500,
    "demand_rules": [...],
    "zone_surge_thresholds": [...],
    "battery_discount_tiers": [...]
  }' \
  "http://localhost:8080/api/v1/admin/config"
```

**Config takes effect within 30 seconds via hot-reload.**

### Available Zones

- `jakarta_pusat` (Central Jakarta)
- `jakarta_selatan` (South Jakarta)
- `jakarta_barat` (West Jakarta)
- `jakarta_timur` (East Jakarta)
- `jakarta_utara` (North Jakarta)
- `bogor`, `depok`, `tangerang`, `bekasi` (satellite cities)

## Key Decisions

### 1. Duration_hours as kWh (Not Time)

**Decision:** Parameter named `duration_hours` (per test requirement) but represents **kWh energy consumed**, not hours.

**Reasoning:**
- Electrum operates battery swap stations (not hourly rentals)
- Pricing is per kWh energy charged, not per hour rented
- Max 1.8 kWh per swap (battery capacity of H1/H3 models)
- Research confirmed this aligns with real Electrum business model

**Trade-off:** Parameter name misleading but matches test spec. Documented clearly for reviewers.

### 2. Go + PostgreSQL

**Why Go:**
- Performance: <100ms p95 requirement met easily
- Concurrency: Goroutines for hot-reload + fleet simulation
- Deployment: Single binary to Cloud Run

**Why PostgreSQL:**
- JSONB for flexible pricing config storage
- Append-only audit trail with atomic writes
- ACID transactions for config updates
- Sufficient for scale (10K vehicles, 500 req/s)

**Alternatives considered:**
- Python: Simpler, but slower for <100ms latency requirement
- TypeScript: Node.js cold starts problematic on Cloud Run
- Redis: Unnecessary for caching; in-memory suffices

### 3. In-Memory Cache (No Redis)

**Decision:** Pricing config, events, and tiers cached in memory with polling refresh.

**Reasoning:**
- Data is tiny (<100KB total across all caches)
- Single Cloud Run instance (no cache coherency needed)
- Polling (30s config/events, 5min tiers) matches requirements
- Zero-ops: no external cache infrastructure

**Trade-off:** Multi-instance deployment would need Redis (documented in SCALABILITY.md).

### 4. A/B Testing as Config Routing (Not Multiplier)

**Decision:** A/B segment determines which full pricing config version is loaded.

**Reasoning:**
- Matches real A/B testing: compare two strategies holistically
- Not just testing one multiplier factor
- Allows testing different demand curves, base prices simultaneously
- Hash-based deterministic assignment (same user always sees same variant)

**Implementation:** `SHA256(user_id) % 100` → segment (0-49=control, 50-99=variant)

### 5. HMAC-SHA256 for Audit Trail Tamper Detection

**Decision:** Every audit entry signed with `HMAC-SHA256(quote_id + user_id + vehicle_id + zone + final_price + created_at, secret)`.

**Reasoning:**
- Tamper-evident: any modification invalidates signature
- No cryptographic key management needed (symmetric, stored as env var)
- Lightweight: <1ms overhead per calculation

**Trade-off:** Not cryptographically bulletproof (key could be compromised), but sufficient for 72h test + audit trail integrity.

## Pricing Formula

```
final_price = base_rate_per_kwh 
            × demand_multiplier(hour, day_of_week)
            × zone_surge_factor(zone_utilization)
            × battery_discount_factor(soc)
            × event_discount(zone, time)
            × loyalty_discount(subscription_tier)
            × duration_hours (kWh)
```

**Example:** Peak hour, high-demand zone, gold subscriber, 0.9 kWh swap, low battery
```
4000 × 1.3 (peak) × 1.5 (surge) × 0.85 (battery) × 1.0 (no event) × 0.9 (gold) × 0.9 kWh
= 5,963 Rp
```

## AI Tool Usage

**Tools Used:** Claude Code (primary)

**What Worked Well:**
- Generating boilerplate (HTTP handlers, DB queries, test scaffolds)
- Refactoring suggestions (identified zone_surge bug, fixed signature)
- Model type generation from requirements
- Test case expansion

**What I Had to Fix:**
- Zone surge calculation: AI hardcoded `0.8` utilization instead of accepting parameter → Fixed to use request parameter
- Migration ON CONFLICT clauses: AI syntax slightly off for PostgreSQL → Corrected manually
- HMAC signing: AI used wrong field concatenation order → Verified and corrected
- Fleet state lookup in handlers: AI didn't fetch from DB → Added explicit query

**Lessons:** AI excellent for scaffolding, needs domain expertise review for correctness (especially DB queries, cryptographic operations).

## What Not Submitted

- Copy-pasted code (none)
- Over-engineered features (kept minimal for 72h scope)
- Non-running code (all builds and tests pass)

## Further Development

With more time:
- Implement A/B config versioning API (create/activate/compare configs)
- Real IoT integration (consume actual battery swap events)
- WebSocket for real-time alerts
- Grafana dashboard + Prometheus metrics
- Multi-region Cloud Run deployment with Redis cache
- Advanced analytics (price elasticity, demand forecasting)
