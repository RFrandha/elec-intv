# Architecture Overview

## System Design

This pricing engine calculates battery swap energy pricing for Electrum's EV fleet using composable factors. Designed for <100ms p95 latency, hot-reloadable configuration, and transparent audit trails.

## Components

### HTTP API Layer (`internal/interfaces/http/`)
- Gin HTTP framework with route groups
- API key authentication (read-only + admin tiers)
- Rate limiting middleware (per IP, 100/min public, 10/min admin)
- Routes to pricing service for calculations
- Request validation (zone whitelist, kWh bounds)

### Application Service (`internal/application/`)
- Orchestrates pricing calculation flow
- Reads from in-memory cache (config, tiers, events, AB test configs)
- Applies factor chain sequentially (demand → zone surge → battery → event → loyalty)
- Signs audit entries with HMAC-SHA256
- A/B config routing: loads different config per segment
- Stores breakdowns for quote retrieval

### Database Layer (`internal/infrastructure/database/`)
- PostgreSQL connection pooling (lib/pq)
- Auto-runs migrations on startup
- Seeds default data (tiers, config, vehicles, AB test configs)
- Schema: `pricing.*` (10 tables)

### Fleet Simulator (`internal/infrastructure/fleet/`)
- Manual refresh via `POST /admin/fleet/refresh`
- Simulates realistic utilization patterns (peak/off-peak, weekdays/weekends)
- Provides zone utilization for pricing calculations
- Deterministic variation (sinusoidal) ensures predictable demos

### Configuration Manager (`internal/application/`)
- Manual refresh via `POST /admin/config/refresh`
- Reads config, events, tiers, and AB test configs from DB
- Stores in in-memory cache with `sync.RWMutex`
- Auto-refresh code exists (30s polling) but disabled for cold-start optimization

## Data Flow

### Pricing Request (Happy Path)
```
1. HTTP request → validation middleware
2. API key verified (read-only tier)
3. Query vehicles table → get vehicle details + current_user_id
4. Query users table → get subscription_tier_id
5. Query fleet_state table → get zone utilization
6. pricing engine.Calculate() → apply factors → compute final price
7. Sign audit entry → write to pricing_audit
8. Return pricing response (30s TTL)
```

### Config Update (Hot-Reload)
```
1. Admin sends PUT /admin/config
2. Handler validates config
3. Write new version to pricing_configs table
4. HTTP response (will activate within 30s)
5. Background poller detects version change
6. Atomic swap: engine.UpdateConfig(newConfig)
7. Next request uses new config
```

### A/B Test Assignment (Config Routing)
```
1. user_id extracted from vehicle lookup
2. SHA256(user_id) % 100 → segment number (control or variant)
3. Look up ab_test_configs table for this segment's config_id
4. Load that config version from pricing_configs table
5. Use that config for ALL factor calculations (demand, zone, battery, event, loyalty)
6. Segment + config version logged in audit trail
```

## Database Schema

### Core Tables (10 total)

**`pricing.tiers`** — Subscription tiers with discount rates
```sql
id TEXT PRIMARY KEY,                          -- 'normal', 'gold'
discount_multiplier NUMERIC(5,2) NOT NULL     -- 1.0 = no discount, 0.9 = 10% off
```

**`pricing.users`** — Subscribers linked to vehicles
```sql
user_id TEXT PRIMARY KEY,
subscription_tier_id TEXT REFERENCES tiers(id),
rental_count INT DEFAULT 0
```

**`pricing.vehicles`** — Fleet vehicles with current state
```sql
vehicle_id TEXT PRIMARY KEY,
zone TEXT NOT NULL,
current_soc NUMERIC(5,2),                    -- battery state of charge (0-100)
current_user_id TEXT REFERENCES users(user_id),
last_swap_timestamp TIMESTAMPTZ
```

**`pricing.fleet_state`** — Zone utilization snapshots
```sql
zone TEXT PRIMARY KEY,
total_vehicles INT,
available_vehicles INT,
bss_count INT,                               -- Battery Swap Stations in zone
updated_at TIMESTAMPTZ
```

**`pricing.pricing_configs`** — Versioned pricing configurations (JSONB)
```sql
config_id SERIAL PRIMARY KEY,
version INT UNIQUE,
config_jsonb JSONB NOT NULL,
created_by TEXT DEFAULT 'system'
```

**`pricing.active_config`** — Singleton pointing to active config version
```sql
id INT PRIMARY KEY DEFAULT 1,                -- always 1
config_id INT REFERENCES pricing_configs(config_id)
```

**`pricing.events`** — Promotional discounts
```sql
event_id SERIAL PRIMARY KEY,
name TEXT,
zone TEXT,                                   -- NULL = global
bss_id INT,                                  -- NULL = zone-wide
start_time TIMESTAMPTZ,
end_time TIMESTAMPTZ,
discount_multiplier NUMERIC(5,2)            -- 0.8 = 20% off
```

**`pricing.ab_test_configs`** — A/B test segment assignments
```sql
test_id SERIAL PRIMARY KEY,
test_name TEXT,
segment_name TEXT,                          -- 'control', 'variant'
config_id INT REFERENCES pricing_configs(config_id),
is_active BOOLEAN DEFAULT true
```

**`pricing.pricing_audit`** — Append-only, HMAC-signed audit trail
```sql
quote_id UUID PRIMARY KEY,
vehicle_id TEXT NOT NULL,
user_id TEXT NOT NULL,
zone TEXT NOT NULL,
kwh_consumed NUMERIC(5,2),
final_price NUMERIC(12,2),
ab_segment TEXT,
hmac_signature TEXT NOT NULL,               -- tamper detection
created_at TIMESTAMPTZ
```

### Indexes
- `pricing_audit (user_id, created_at)` — user lookup
- `pricing_audit (created_at)` — time-range queries
- `pricing_audit (ab_segment)` — A/B analysis
- `events (start_time, end_time)` — active event lookup
- `fleet_state (zone)` — fast zone lookup

## In-Memory Caching Strategy

**What is cached:**
- Active pricing config (JSONB)
- Active events (upcoming)
- Subscription tiers (2 rows: normal, gold)

**Refresh schedule:**
- Pricing config + events: every 30 seconds
- Tiers: every 5 minutes

**Thread safety:**
- All cache updates via `sync.RWMutex`
- Read path: `RLock()` (concurrent reads allowed)
- Write path: `Lock()` (exclusive write)

**Why no Redis:**
- Data fits in <100KB memory
- Single Cloud Run instance (no coherency needed)
- Zero-ops, no infrastructure cost
- 30s poll cadence matches requirements

## A/B Testing: Config Routing (Not Multiplier)

**Design decision:** A/B segment determines which full pricing config version is loaded, not an additional multiplier.

**Why:** Matches real A/B testing (test two strategies holistically), not just testing single multiplier.

**Flow:**
```
request → derive user_id from vehicle → SHA256(user_id) % 100
  ├─ < 50 → load config_v1 (control)
  └─ >= 50 → load config_v2 (variant)
```

**Example configs:**
- Control: base_price=4000, demand_peak=1.3
- Variant: base_price=4500, demand_peak=1.2

Result: Different base rates + different demand curves → different final prices.

## Deployment Architecture (GCP)

```
┌─────────────────────────────────────────────────────────┐
│  Cloud Run Service (asia-southeast1)                    │
│  ├─ Auto-scales 1-10 instances                         │
│  ├─ Stateless pricing API                              │
│  └─ In-memory cache (no external cache)                │
└────────────────┬────────────────────────────────────────┘
                 │ Unix socket
┌────────────────▼────────────────────────────────────────┐
│  Cloud SQL (Postgres)                                   │
│  ├─ db-f1-micro (0.6GB RAM, 10GB disk)                 │
│  ├─ Schema: pricing.*                                   │
│  └─ Private IP (no public endpoint)                    │
└─────────────────────────────────────────────────────────┘
```

## Key Trade-offs

| Decision | Why This Way | Alternative |
|----------|-------------|-------------|
| Go language | Performance <100ms, goroutines, single binary | Python (slower), TypeScript (cold starts) |
| PostgreSQL | JSONB configs, ACID, sufficient scale | MySQL (less flexible JSONB) |
| In-memory cache | Zero-ops, <100KB data, 30s latency ok | Redis (costs $30/mo, overkill) |
| A/B config routing | Holistic strategy testing | Single multiplier (less realistic) |
| HMAC audit trail | Tamper-evident, lightweight | Full cryptographic signatures (heavier) |
| Duration_hours = kWh | Matches test spec, documents assumption | Rename to kwh_consumed (violates spec) |

## Future Considerations (With More Time)

1. **Multi-instance caching:** Add Redis for cache coherency with multi-instance Cloud Run
2. **Real-time alerts:** WebSocket for BSS anomaly notifications
3. **Advanced analytics:** Price elasticity models, demand forecasting
4. **Payment integration:** Stripe/Gopay for actual booking transactions
5. **A/B test management UI:** Dashboard to create/monitor/analyze tests
6. **Prometheus metrics:** `/metrics` endpoint for Grafana dashboards
7. **Territory-based pricing:** Different base rates per city/province
