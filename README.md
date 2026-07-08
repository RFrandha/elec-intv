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
- Go 1.25+ (for local development)

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
│  HTTP API Layer (Go + Gin)                              │
│  ├─ GET /health                                        │
│  ├─ GET /api/v1/pricing (read-only/admin key)          │
│  ├─ GET /api/v1/pricing/{quote_id}/breakdown           │
│  ├─ Admin endpoints (config, events, fleet, analytics) │
│  └─ Middleware: API key auth, rate limiting             │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│  Application Service (internal/application)               │
│  ├─ Factor chain (demand, zone, battery, event, loyalty)│
│  ├─ Config hot-reload (on-demand refresh)               │
│  ├─ HMAC audit trail (tamper-evident)                   │
│  └─ A/B testing (config routing per segment)            │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│  Infrastructure (internal/infrastructure)                 │
│  ├─ database/ (PostgreSQL repos)                        │
│  ├─ cache/ (in-memory with RWMutex)                     │
│  └─ fleet/ (simulator, manual refresh)                  │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│  PostgreSQL (Cloud SQL)                                  │
│  ├─ pricing.vehicles (fleet state)                      │
│  ├─ pricing.pricing_configs (versioned JSONB)           │
│  ├─ pricing.pricing_audit (append-only, HMAC-signed)    │
│  ├─ pricing.events (promotions)                         │
│  ├─ pricing.tiers (loyalty tiers)                       │
│  ├─ pricing.users (subscribers)                         │
│  └─ pricing.ab_test_configs (segment→config mapping)    │
└─────────────────────────────────────────────────────────┘
```

## API Overview

### Health Check

```bash
curl -s "https://<host>/health"
# {"status":"ok"}
```

### Pricing Endpoint

**Request:**
```bash
curl -H "X-API-Key: <READ_ONLY_API_KEY>" \
  "http://localhost:8080/api/v1/pricing?vehicle_id=V001&zone=jakarta_pusat&duration_hours=0.9"
```

**Response:**
```json
{
  "quote_id": "550e8400-e29b-41d4-a716-446655440000",
  "vehicle_id": "V001",
  "zone": "jakarta_pusat",
  "kwh_consumed": 0.9,
  "base_rate_per_kwh": 4000,
  "final_price": 3240,
  "valid_until": "2026-07-08T05:53:25Z",
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
curl -H "X-API-Key: <READ_ONLY_API_KEY>" \
  "http://localhost:8080/api/v1/pricing/550e8400-e29b-41d4-a716-446655440000/breakdown"
```

Shows each factor's contribution to final price.

### Admin Endpoints

**Get Current Config:**
```bash
curl -H "X-API-Key: <ADMIN_API_KEY>" \
  "http://localhost:8080/api/v1/admin/config"
```

**Update Config:**
```bash
curl -X PUT -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "base_price": 4500,
    "demand_rules": [...],
    "zone_surge_thresholds": [...],
    "battery_discount_tiers": [...]
  }' \
  "http://localhost:8080/api/v1/admin/config"
```

**Refresh Config (after update):**
```bash
curl -X POST -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Length: 0" \
  "http://localhost:8080/api/v1/admin/config/refresh"
```

### Available Zones

- `jakarta_pusat` (Central Jakarta)
- `jakarta_selatan` (South Jakarta)
- `jakarta_barat` (West Jakarta)
- `jakarta_timur` (East Jakarta)
- `jakarta_utara` (North Jakarta)
- `bogor`, `depok`, `tangerang`, `bekasi`, `bandung` (satellite cities)

## Configuration Management

### Update Config
```bash
curl -X PUT -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{"base_price": 4500, "demand_rules": [...]}' \
  "https://<host>/api/v1/admin/config"
```

### Manual Refresh (Post-Update)
After updating config, refresh the in-memory cache manually:

```bash
# Refresh config, events, and tiers:
curl -X POST -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Length: 0" \
  "https://<host>/api/v1/admin/config/refresh"

# Refresh fleet state utilization:
curl -X POST -H "X-API-Key: <ADMIN_API_KEY>" \
  -H "Content-Length: 0" \
  "https://<host>/api/v1/admin/fleet/refresh"
```

### Config Hot-Reload Strategy
- **Scheduled auto-refresh code exists** (30s polling for config/events, 5min for tiers, 30s fleet simulation)
- **Currently disabled** on startup to avoid keeping Cloud Run alive (cold-start optimization)
- **Manual refresh via `POST /admin/config/refresh` and `POST /admin/fleet/refresh`** when needed
- To enable auto-refresh: uncomment `StartCacheUpdater()` and `simulator.Start()` in `src/cmd/server/main.go`

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

**How it works:**
- `SHA256(user_id) % 100` → segment (0-49=control, 50-99=variant)
- `ab_test_configs` table maps each segment to a config version
- Service loads the specific config version for that segment
- Example: control gets config_v1 (base=4000), variant gets config_v2 (base=4500)
- Segment logged in audit trail for post-hoc analysis

**Why this matters:**
- Compare two pricing *strategies* holistically (not just one multiplier)
- Different base prices, demand rules, and battery discounts per segment
- Deterministic: same user always sees same variant

**Implementation:** `SHA256(user_id) % 100` → segment (0-49=control, 50-99=variant)

### 5. HMAC-SHA256 for Audit Trail Tamper Detection

**Decision:** Every audit entry signed with `HMAC-SHA256(quote_id + user_id + vehicle_id + zone + final_price + created_at, secret)`.

**Reasoning:**
- Tamper-evident: any modification invalidates signature
- No cryptographic key management needed (symmetric, stored as env var)
- Lightweight: <1ms overhead per calculation

**Trade-off:** Not cryptographically bulletproof (key could be compromised), but sufficient for 72h test + audit trail integrity.

### 6. Config Hot-Reload Strategy

**Decision:** Manual refresh via admin API (scheduled auto-refresh code exists but disabled).

**Reasoning:**
- Cloud Run cold-start: background goroutines keep the instance alive, defeating auto-scaling-to-zero
- Code for 30s polling (config/events/tiers) and 30s fleet simulation already exists in the codebase
- Manually triggered via `POST /admin/config/refresh` and `POST /admin/fleet/refresh`

**To enable auto-refresh:** Uncomment these two lines in `src/cmd/server/main.go`:
```go
pricingService.StartCacheUpdater()  // line 42
simulator.Start()                    // line 45
```

**Trade-off:** Manual refresh adds one extra step after config updates. Acceptable for 72h test. In production, auto-refresh is standard.

## Pricing Formula

```
final_price = base_rate_per_kwh 
            × demand_multiplier(hour, day_of_week)
            × zone_surge_factor(zone_utilization)
            × battery_discount_factor(return_soc)
            × event_discount(zone, time)
            × loyalty_discount(subscription_tier)
            × duration_hours (kWh)
```

For a detailed explanation of each multiplier and how they work together, see [Pricing Calculation Walkthrough](docs/UBIQUITOUS_LANGUAGE.md#pricing-calculation-walkthrough).

**Example:** Peak hour, high-demand zone, gold subscriber, 0.9 kWh swap, 50% return SoC
```
4000 × 1.3 (peak) × 1.5 (surge) × 0.90 (return SoC) × 1.0 (no event) × 0.9 (gold) × 0.9 kWh
= 5,686 Rp
```

## Documentation

Full documentation available in the `docs/` directory:

| Document | Description |
|----------|-------------|
| `PRD.md` | Product Requirements Document — user stories, decisions, scope |
| `ARCHITECTURE.md` | System design, data flow, database schema, trade-offs |
| `SECURITY.md` | Authentication, input validation, rate limiting, HMAC audit |
| `SCALABILITY.md` | Scaling plan, connection pooling, cost projections |
| `UBIQUITOUS_LANGUAGE.md` | Domain glossary — terms, definitions, relationships |
| `TEST_SCENARIO.md` | Complete test suite with curl commands and expected responses |
| `API_DOCUMENTATION.md` | Full API reference with endpoint docs, examples, and error codes |

## AI Tool Usage

**Tool:** opencode (free tier model)

**Primary CLI:** opencode — interactive agent for software engineering tasks

**Skills Used (Matt Pocock agentic skills):**
- `grill-me` — Stress-test architecture decisions before building
- `to-prd` — Synthesize conversation into PRD
- `ubiquitous-language` — Extract domain terminology, build UBIQUITOUS_LANGUAGE.md
- `tdd` — Drive 10 TDD cycles in vertical slices
- `design-an-interface` — Explore API shape alternatives
- `domain-modeling` — Sharpen terminology and relationships

**What Worked Well:**
- Scaffolding entire DDD project structure (Layered: interfaces → application → infrastructure → domain)
- Generating Gin HTTP handlers with middleware and validation
- Writing PostgreSQL migrations with JSONB, indexes, and constraints
- 10 TDD cycles: each cycle generated tests, stub code, then all green
- Refactoring prompts (extracted config validation, zone whitelist)
- Test expansion (edge cases, error paths, determinism)

**What I Had to Fix:**
- Zone surge hardcoded `0.8` instead of parameter → Fixed to use request utilization
- Migration ON CONFLICT syntax → Corrected for PostgreSQL dialect
- Migration idempotency → Changed to `WHERE NOT EXISTS` (Postgres ignores `UNIQUE` on `CREATE TABLE IF NOT EXISTS` when table exists)
- Quote ID format → Changed from `Q-<hex>` prefix to full UUID (UUID column type)
- Audit write failing → Added `UserID` to user fallback struct (was empty string)
- Event creation via curl → `curl.exe` on Windows mangled JSON; used PowerShell `Invoke-WebRequest`
- Rate limiting on admin → Added `Content-Length: 0` for POST requests on Cloud Run
- `bandung` zone missing → Added to valid zones whitelist
- Battery discount perspective → Flipped from pickup SoC to return SoC for BSS efficiency

**Key Insight:** AI excellent for scaffolding, writing tests, and generating consistent boilerplate. Requires domain expertise review for correctness (DB queries, cryptographic ops, idempotency, business logic). PowerShell vs. curl incompatibility on Windows needs human awareness.

**Note:** All code reviewed before commit. No secrets in codebase (keys stored in env vars, excluded PDF).

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
