# PRD: Dynamic Pricing Engine for Electrum EV Battery Swaps

## Problem Statement

Electrum currently uses **static pricing** (Rp 4,000/kWh equivalent) for battery swap energy, regardless of demand, zone utilization, or operational conditions. This leads to:

- Revenue loss during peak demand periods (underselling)
- Poor fleet rebalancing (vehicles with idle batteries sit unused when they could be discounted)
- No operational flexibility for zone-based surge pricing or promotional events
- No ability to test pricing strategies (A/B testing)

The business needs a **dynamic pricing system** that considers demand patterns, zone utilization, battery conditions, and events while maintaining **transparency** (price breakdowns) and **audit integrity** (tamper-evident logs).

## Solution

A **dynamic pricing engine API** that:

1. Calculates battery swap energy pricing using composable factors (demand, zone surge, battery discount, events, loyalty, A/B testing)
2. Provides transparent price breakdowns showing each factor's contribution
3. Allows admins to update pricing configurations without redeployment (hot-reload within 30 seconds)
4. Maintains an immutable, tamper-evident audit trail (HMAC-signed) of all price calculations
5. Supports A/B testing of pricing strategies across user segments (hash-based deterministic assignment)
6. Simulates realistic fleet utilization across Jabodetabek zones for demonstration

**Target metrics:**
- <100ms p95 latency for pricing calculations
- 500 requests/second peak capacity
- Configuration updates take effect within 30 seconds
- 10,000+ vehicles supported
- Append-only audit trail with tamper detection

## User Stories

### Pricing Calculation (Public API)

1. As a **BSS station system**, I want to request a price quote for a battery swap by vehicle_id and zone, so that I can charge the renter correctly
2. As a **renter**, I want to see a breakdown of how my swap price was calculated (each factor's input and multiplier), so that I understand pricing transparency
3. As a **BSS system**, I want price quotes to have a 30-second TTL, so that renters can't game the system by holding old quotes
4. As a **renter with low battery SoC**, I want automatic discounts applied to my swap price, so that I'm incentivized to return depleted batteries and help rebalance the fleet
5. As a **renter during peak hours**, I want to see demand-based pricing increases (e.g., 1.3x from 5-7 PM on weekdays), so that I'm aware of high-demand pricing
6. As a **gold-tier subscriber**, I want automatic 10% loyalty discounts applied to all swaps, so that I'm rewarded for my subscription
7. As a **renter in a high-utilization zone**, I want surge pricing (e.g., 1.5x when zone utilization >80%), so that I understand zone-specific pricing

### Configuration Management (Admin API)

8. As an **operations admin**, I want to update the base price without redeploying, so that I can respond to market conditions quickly
9. As an **operations admin**, I want to adjust demand multiplier schedules (peak hours, weekends) via configuration, so that I can optimize revenue by time of day
10. As an **operations admin**, I want to configure zone surge thresholds dynamically, so that I can balance demand across zones
11. As an **operations admin**, I want configuration changes to take effect within 30 seconds, so that I don't need to wait for deployments
12. As an **operations admin**, I want configuration validation to prevent invalid settings (negative prices, conflicting rules), so that I don't break the pricing system
13. As an **operations admin**, I want to view configuration change history with timestamps and who made changes, so that I can audit pricing decisions
14. As an **operations admin**, I want to roll back to a previous configuration version, so that I can quickly recover from pricing mistakes

### Event Management (Admin API)

15. As a **marketing admin**, I want to create time-limited promotional events with specific discount multipliers, so that I can run campaigns
16. As a **marketing admin**, I want to scope events to specific zones, so that I can target promotions to high-value areas
17. As a **marketing admin**, I want to scope events to specific BSS locations, so that I can drive traffic to underutilized stations
18. As a **marketing admin**, I want to see which events are currently active and when they expire, so that I can verify campaigns are running
19. As a **marketing admin**, I want to cancel events before they expire, so that I can stop campaigns early if needed

### Audit & Compliance

20. As a **compliance officer**, I want every price calculation logged with all inputs (vehicle, zone, soc, tier) and all factors applied, so that I can investigate customer complaints
21. As a **compliance officer**, I want audit logs to be append-only with HMAC signatures, so that I can trust data integrity (no tampering)
22. As a **compliance officer**, I want to retrieve the full audit trail for a specific vehicle or user within a date range, so that I can conduct investigations
23. As a **customer support agent**, I want to look up a specific price quote and see exactly why a customer received that price, so that I can explain it clearly
24. As a **data analyst**, I want to query historical pricing data (average price by zone, demand patterns, A/B test results), so that I can optimize pricing strategy

### A/B Testing

25. As a **product manager**, I want to run A/B tests where different user segments get different pricing configurations, so that I can validate hypotheses before full rollout
26. As a **product manager**, I want A/B segment assignment to be deterministic (same user always gets same segment), so that users have a consistent experience
27. As a **product manager**, I want to see A/B test results (requests per segment, average price per segment, total revenue per segment), so that I can decide which configuration won
28. As a **data scientist**, I want the A/B segment assignment logged in the audit trail, so that I can perform post-hoc analysis and validate statistical significance

### Fleet Management

29. As an **operations admin**, I want to see current fleet utilization by zone (available vs total vehicles), so that I can understand capacity and plan deployments
30. As an **operations admin**, I want to see which zones are approaching surge thresholds, so that I can proactively rebalance fleet
31. As a **developer/reviewer**, I want the demo system to simulate realistic fleet utilization patterns, so that I can see dynamic pricing in action

### API Access & Security

32. As an **API consumer**, I want rate limiting on pricing endpoints, so that no single client can overwhelm the system
33. As an **operations admin**, I want admin endpoints protected by API key, so that only authorized personnel can modify configurations
34. As a **reviewer/tester**, I want clear API documentation with sample curl commands, so that I can quickly explore the system
35. As a **developer**, I want the system to run locally with one command (`docker compose up`), so that I can test locally
36. As a **developer**, I want the system deployable to GCP Cloud Run with IaC (cloudrun.yaml), so that I can see it in production

## Implementation Decisions

### Technology Stack

- **Language: Go** — Performance (<100ms p95), goroutines for concurrency (hot-reload, fleet sim), single binary deployment
- **Database: PostgreSQL (Cloud SQL)** — JSONB for flexible pricing configs, append-only audit logs, ACID for transactional integrity
- **Deployment: GCP Cloud Run + Cloud SQL** — Serverless auto-scaling, minimal ops overhead, IaC via cloudrun.yaml
- **Local dev: Docker Compose** — One-command setup, Postgres container, matches production behavior

### Pricing Formula

```
final_price = base_rate_per_kwh 
            × demand_multiplier(hour, day_of_week)
            × zone_surge_factor(zone_utilization)
            × battery_discount_factor(soc)
            × event_discount(zone, time)
            × loyalty_discount(subscription_tier)
            × duration_hours(kWh)
```

All factors are **multipliers** (commutative, composable). No additive components.

### Duration_hours Parameter (Test Requirement Alignment)

- Parameter name: `duration_hours` (per test spec)
- **Actual meaning: kWh energy consumed** (per Electrum business model)
- Range: 0.1-1.8 kWh (max battery capacity)
- Rationale: Electrum operates battery swaps (not hourly rentals). Pricing is per kWh energy, not per hour. Research confirmed real battery capacity is 1.8 kWh.
- Trade-off: Parameter name misleading but matches test requirement. Documented clearly for reviewers.

### A/B Testing: Config Routing (Not Multiplier)

- **Design:** A/B segment determines which full pricing config version is loaded
- **Not:** A single additional multiplier
- **Rationale:** Tests two pricing strategies holistically (different base prices, different demand curves), not just one factor
- **Implementation:** Hash-based deterministic assignment (`SHA256(user_id) % 100`); control segment (0-49) vs variant (50-99)

### In-Memory Caching (No Redis)

- **What:** Pricing config, active events, subscription tiers cached in memory
- **Why:** Data is tiny (<100KB), single Cloud Run instance (no cache coherency needed), 30s poll cadence acceptable
- **Refresh:** Config/events every 30s, tiers every 5min
- **Trade-off:** Multi-instance deployment would need Redis (documented for future scaling)

### HMAC-SHA256 Audit Trail

- **Signing:** Each audit entry signed with `HMAC-SHA256(quote_id + user_id + vehicle_id + zone + final_price + created_at, secret)`
- **Why:** Tamper-evident (any modification invalidates signature), lightweight (<1ms overhead), sufficient for 72h test
- **Trade-off:** Not cryptographically bulletproof (symmetric key), but acceptable for audit trail integrity

### Tier Discount Logic

- **Direction:** Lower multiplier = more discount (0.9 = 10% off, not 1.1)
- **Default tiers:** Normal (1.0, no discount), Gold (0.9, 10% off)
- **Extensible:** New tiers added by row insert to `tiers` table; system picks up via 5min cache refresh

## Testing Decisions

### Test Philosophy

- **Good tests** verify behavior through public API, not implementation details
- **Good tests** survive refactors (code changes, but behavior unchanged)
- **Good tests** use independent expected values (not recomputed from code)
- **Tautological tests** fail: asserting a value equals itself (e.g., `expect(add(a, b)).toBe(a + b)`)

### Test Seam

- **Primary:** HTTP handlers (integration tests via HTTP requests)
- **Supporting:** Unit tests for factor calculations, config validation, HMAC signing
- **Why:** Integration tests cover full stack; internal refactors don't break them

### 10 TDD Cycles (Vertical Slices)

| Cycle | Test | Behavior Verified |
|-------|------|------------------|
| 1 | Basic pricing returns 200 + quote_id | Tracer bullet: HTTP → engine works |
| 2-4 | Battery discount, demand multiplier, zone surge | Each factor applies correctly |
| 5-6 | Loyalty discount, events | Tier lookup, event matching |
| 7 | A/B segment routing | Control vs variant configs differ |
| 8 | Breakdown endpoint | Transparency API |
| 9 | Config hot-reload | 30s polling takes effect |
| 10 | Admin analytics, error cases | Aggregation, robustness |

## Out of Scope

1. **Real IoT integration** — Fleet state is simulated, not from real BSS devices
2. **Battery-vehicle historical mapping** — Assumes vehicle_id provided; BSS→vehicle lookup is separate system
3. **Payment processing** — Pricing engine only; no transaction handling
4. **Booking/reservation enforcement** — Quote validity (30s TTL) documented, not enforced at booking
5. **Mobile app** — API designed for mobile, no actual app built
6. **Terraform** — Using Cloud Run YAML (still IaC)
7. **GCP Secret Manager** — Using Cloud Run env vars (simpler for 72h test)
8. **Multi-region deployment** — Single region (asia-southeast1)
9. **Advanced observability** — Basic logging only (Prometheus mentioned as future)
10. **Multi-tier pricing** — Only normal/gold tiers initially (system is extensible)

## Further Notes

### Electrum Domain Context (Research-Backed)

- **Real zones:** Jabodetabek (Jakarta + Bogor, Depok, Tangerang, Bekasi)
- **350+ Battery Swap Stations (BSS)** across region
- **Real baseline pricing:** Rp 45,000/day rent-to-own (Sewa Milik), but pricing engine targets swap energy costs
- **Battery specs:** 1.8 kWh lithium NCM (H1/H3 models)
- **Swap flow:** Renter inserts battery → BSS reads battery_id → system determines vehicle/user → calculates price

### AI Tool Usage (for README)

Document which AI tools were used, what worked well, and what had to be fixed manually.

### Scalability (for SCALABILITY.md)

- 10K vehicles, 500 req/s peak, <100ms latency
- Horizontal scaling via Cloud Run auto-scaling
- Config updates within 30 seconds via polling
- Audit log partitioned by month for data growth

### Security (for SECURITY.md)

- API key auth (two tiers: read-only, admin)
- Input validation (zone whitelist, kWh bounds)
- HMAC audit trail (tamper-evident)
- Rate limiting per IP
- SQL injection prevention (parameterized queries)
