# Scalability Plan

## Scale Assumptions

Based on Electrum's real operations:
- **10,000+ vehicles** active across Jabodetabek
- **350+ Battery Swap Stations (BSS)**
- **~16M battery swaps** completed (historical)
- **Peak throughput** for pricing API: ~500 requests/second
- **Sustained throughput**: ~166 requests/second (10K vehicles × 1 swap/60s)

## 1. Concurrent Users

### Current Architecture (In-Memory Cache, Single Instance)

**Capacity:** Single Cloud Run instance (1 vCPU, 512MB RAM):
- Handles ~200 req/s sustained
- <10ms p99 for cache-hit pricing calculations
- Database connection pool: 25 connections

**Horizontal Scaling (Cloud Run default):**
- Cloud Run auto-scales to 100 instances max
- Each instance: 200 req/s → theoretical 20,000 req/s total
- No cache coherency issue: in-memory cache refreshes within 30s

**Bottleneck:** Cloud SQL max connections
- db-f1-micro: 25 max connections
- Solution: Use connection pooler (PgBouncer) or larger instance tier
- With PgBouncer: 25 connections can proxy hundreds of concurrent queries

### Connection Pool Sizing

| Connection | Max Connections | Pricing Capacity |
|------------|----------------|-----------------|
| db-f1-micro (shared) | 25 | ~250 req/s |
| db-custom-1-3840 | 50 | ~500 req/s |
| db-custom-2-7680 | 100 | ~1,000 req/s |
| db-custom-4-15360 | 200 | ~2,000 req/s |

**Recommendation:** db-custom-1-3840 for production (50 connections × 10 req/s each = 500 req/s)

## 2. Data Growth

### Audit Trail Growth

**Per-request storage:** ~500 bytes per audit entry (JSON + signature)

| Scale | Volume | Storage/Month |
|-------|--------|--------------|
| Current (166 req/s) | ~14.3M req/month | ~7.2 GB/month |
| Peak (500 req/s) | ~43M req/month | ~21.5 GB/month |

**Strategy: Table Partitioning by Month**

```sql
-- Using PostgreSQL table inheritance or pg_partman
CREATE TABLE pricing.pricing_audit (
  LIKE pricing.pricing_audit_template INCLUDING DEFAULTS
) PARTITION BY RANGE (created_at);

CREATE TABLE pricing.pricing_audit_2026_07
  PARTITION OF pricing.pricing_audit
  FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
```

**Retention Policy**
- Month-old data: available online
- 3-6 months: migrated to cold storage (Cloud Storage)
- 12 months: archived

### Fleet State Table (Overwrite Pattern)

- `fleet_state` uses overwrite pattern (no history)
- Only 9 rows (one per zone)
- Negligible growth

### Event Table

- Low cardinality: tens of events, not thousands
- Old events automatically excluded by `WHERE end_time > NOW()`

## 3. Peak Load Handling

### Traffic Patterns

Based on Jakarta ride-hailing patterns:

| Time Period | Traffic Volume | Characteristics |
|-------------|---------------|-----------------|
| 00:00-05:00 | 5% of peak | Off-peak, minimal demand |
| 07:00-09:00 | 100% of peak | Morning rush hour |
| 12:00-14:00 | 40% of peak | Lunch break |
| 17:00-19:00 | 100% of peak | Evening rush hour |
| 21:00-23:00 | 30% of peak | After-hours leisure |

### Rate Limiting

Three tiers of rate limiting:
- Public pricing API: 100 req/min per IP
- Admin read endpoints: 10 req/min per key
- Admin write endpoints: 1 req/min per key

### Auto-Scaling Configuration (Cloud Run)

```yaml
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/maxScale: "100"
        autoscaling.knative.dev/target: "80"
        autoscaling.knative.dev/minScale: "1"
  containerConcurrency: 80
```

**Target 80 concurrent requests per instance.** At 500 req/s, this means:
- 500 / 80 = 7 instances at peak
- Each instance handles ~6.25 requests/second
- Room for 25x headroom above baseline (200 req/s)

### Connection Pooling

```go
// pgxpool configuration for production
config.MaxConns = 25
config.MinConns = 5
config.MaxConnLifetime = 30 * time.Minute
config.MaxConnIdleTime = 5 * time.Minute
```

## 4. Horizontal Scaling Approach

### Current Architecture (Stateless)

```
┌──────────┐    ┌──────────┐    ┌──────────┐
│ CloudRun │    │ CloudRun │    │ CloudRun │
│ Instance │    │ Instance │    │ Instance │
│   #1     │    │   #2     │    │   #N     │
└────┬─────┘    └────┬─────┘    └────┬─────┘
     └───────────────┼───────────────┘
                     │ Unix Socket
              ┌──────▼──────┐
              │ Cloud SQL   │
              │ (Postgres)  │
              └─────────────┘
```

**How horizontal scaling works:**
- Each Cloud Run instance is stateless
- In-memory cache refreshes independently (30s poll)
- Shared database ensures data consistency
- Load balancer (Cloud Run built-in) distributes requests

**Cache Coherency:** Not guaranteed across instances (30s window). Acceptable for pricing configuration (30-second max staleness). Events and tiers are even less time-sensitive.

### Read Replicas for Analytics

For non-critical analytics queries (A/B stats, pricing history), use a Cloud SQL read replica:

```
┌──────────┐    ┌──────────┐
│ Price    │    │ Admin    │
│ Instance │    │ Instance │
└────┬─────┘    └────┬─────┘
     │ Write         │ Read
┌────▼──────┐  ┌────▼────────┐
│ Primary   │←→│ Read        │
│ Cloud SQL │  │ Replica     │
└───────────┘  └─────────────┘
```

**Separation of concerns:**
- Pricing instances (high throughput, low latency) → primary DB
- Admin analytics → read replica (unbounded query load)

### Alternative: Redis Cache (For Multi-Instance Coherency)

If instant cache coherency across instances is required, add Redis:

```
┌──────────┐    ┌──────────┐
│ CloudRun │    │ CloudRun │
│   #1     │    │   #2     │
└────┬─────┘    └────┬─────┘
     │               │
     └───────┬───────┘
             │ 60-second TTL
       ┌─────▼──────┐
       │  Redis     │
       │ (Memorystore)│
       └─────┬──────┘
             │ 
       ┌─────▼──────┐
       │ Cloud SQL  │
       └────────────┘
```

**When to use Redis:**
- Multi-region Cloud Run deployment
- Sub-second config propagation needed
- 72h test: NOT needed (in-memory cache is simpler, cheaper, faster)

## Data Migration for Scale

### Read-only to Write Scaling

For analytics endpoints that scan audit data:
- Read replica separates query load from transactional load
- Partitioned tables enable partition-pruning queries
- Materialized views for pre-aggregated A/B stats

### Future: TimescaleDB

For future telemetry-heavy workloads (real-time vehicle tracking):
- TimescaleDB hypertables for time-series optimization
- Automatic chunking and compression
- Continuous aggregates for pre-computed statistics

## Cost Projection at Scale

| Component | db-f1-micro (dev) | db-custom-1 (production) |
|-----------|------------------|-------------------------|
| Cloud Run | ~$5/month | ~$25/month | 
| Cloud SQL | ~$7/month | ~$50/month |
| Redis (if added) | $0 | ~$30/month |
| Total | ~$12/month | ~$105/month |
