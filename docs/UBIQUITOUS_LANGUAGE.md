# Ubiquitous Language

Domain terminology for Electrum Dynamic Pricing Engine. Based on real business research of Electrum's battery swap operations and EV rental model.

## Pricing

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **Base Price** | Standard rate per kWh (Rp 4,000) before any factors applied | Default rate, standard rate |
| **Dynamic Pricing** | Price that varies based on demand, zone, battery, and other factors | Surge pricing, variable pricing |
| **Price Quote** | Calculated price for a specific battery swap, valid for 30 seconds | Quote, estimate, pricing output |
| **Pricing Breakdown** | Per-factor explanation showing each multiplier and its input values | Price decomposition, itemization |
| **Quote TTL** | Duration (30 seconds) that a Price Quote remains valid before expiration | Quote expiry, timeout |
| **Final Price** | Total calculated price after all factors are applied to the Base Price | Total price, total |
| **Battery Discount Factor** | Multiplier applied based on estimated return battery SoC at BSS (0.80 for ≥60%, 0.90 for 40-60%, 1.0 for <40%), calculated as `(1.8 - kWh_consumed) / 1.8 × 100` | SoC discount, return-battery discount |
| **Loyalty Discount** | Multiplier applied based on subscriber tier (0.9 for Gold, 1.0 for Normal) | Tier discount, subscription discount |

## Factors

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **Factor** | A configurable rule that modifies the Base Price by a multiplier | Rule, modifier, adjustment |
| **Multiplier** | Numeric value (e.g., 1.3, 0.85) by which the price is multiplied at a given step | Coefficient, ratio |
| **Demand Multiplier** | Factor based on time-of-day and day-of-week demand patterns (1.3 for weekday 5-7 PM) | Time multiplier, demand factor |
| **Zone Surge Factor** | Factor based on current utilization percentage in a Zone (1.5 at >80% utilization) | Zone multiplier, area surge |
| **Event Discount** | Promotional factor for a time-limited event, optionally scoped to a Zone or BSS | Promo, campaign discount |
| **A/B Segment Factor** | Factor applied conditionally when the Renter belongs to a specific A/B Test Segment | Variant factor, experiment rule |

## Fleet & Zones

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **Vehicle** | An individual electric 2-wheeler in Electrum's fleet, assigned to a current Zone | EV, bike, unit |
| **Fleet** | All Vehicles across all Zones in the Jabodetabek area | Fleet inventory, all vehicles |
| **Zone** | Geographic area in Jabodetabek where Vehicles are located (9 zones total) | Area, region, district |
| **Zone Utilization** | Percentage of Vehicles currently rented out of total Vehicles in a Zone | Occupancy rate, usage rate |
| **Battery Swap Station (BSS)** | Physical station where Renters exchange depleted batteries for charged ones | Swap station, charging point |
| **State of Charge (SoC)** | Current battery level of a Vehicle (0%-100%) | Battery level, charge level |
| **Fleet State** | Snapshot of Zone Utilization across all Zones, updated by the Simulator every 30s | Fleet snapshot, operational state |
| **Last Swap Timestamp** | Timestamp of the most recent battery swap for a Vehicle, used to calculate idle time | Last swap time |

## Configuration

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **Pricing Configuration** | A versioned JSONB object defining Base Price, Factors, thresholds, and rules | Config, pricing ruleset |
| **Config Version** | Auto-incrementing version assigned each time a Pricing Configuration is updated | Revision, schema version |
| **Config Hot-Reload** | Mechanism by which a new Pricing Configuration takes effect within 30 seconds without restarting | Live update, zero-downtime update |
| **Active Config** | Singleton table entry pointing to the currently active Config Version | Current config, live config |

## Audit

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **Audit Trail** | Append-only log table of all Price Quote calculations | Audit log, pricing log |
| **Audit Entry** | Single row in the Audit Trail containing inputs, factors applied, Final Price, and HMAC signature | Audit record, log entry |
| **HMAC Signature** | SHA-256 HMAC hash of an Audit Entry's key fields, enabling tamper detection | Signature, integrity hash |
| **HMAC Secret** | Symmetric key used to generate and verify HMAC Signatures (stored as environment variable) | Signing key, audit secret |

## Events

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **Event** | A time-limited promotional discount, optionally scoped to a Zone or a specific BSS | Promotion, campaign, promo |
| **Active Event** | An Event whose current time falls within its start-to-end window | Live event, ongoing campaign |
| **Event Discount** | The Multiplier applied when an Active Event matches the requested Zone or BSS | Promotional rate, campaign discount |

## A/B Testing

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **A/B Test** | Experiment comparing two Pricing Configurations (Control vs Variant) across user Segments | Experiment, split test |
| **Segment** | Group of Renters assigned to either Control or Variant based on hash of user_id | Bucket, group, cohort |
| **Control** | The default Pricing Configuration in an A/B Test | Baseline, group A |
| **Variant** | The experimental Pricing Configuration in an A/B Test | Treatment, group B |

## Actors

| Term | Definition | Aliases to avoid |
|------|-----------|-----------------|
| **Renter** | Person who rents Electrum Vehicles and swaps batteries at BSS | Customer, rider, end user |
| **Subscriber** | A Renter with a subscription tier (Gold) receiving loyalty discounts | Gold member, member |
| **Operations Admin** | Person who manages Pricing Configurations, Events, and Fleet State | Config admin, pricing manager |
| **Compliance Officer** | Person who verifies Audit Trail integrity and investigates pricing disputes | Auditor, reviewer |
| **API Consumer** | Any system or person that calls the pricing API | Client, caller, integrator |

## Relationships

- A **Vehicle** belongs to exactly one **Zone**
- A **Vehicle** is assigned to exactly one **Renter** (via `current_user_id`)
- A **Renter** has exactly one **Subscription Tier** (Normal or Gold)
- A **Zone** contains many **Vehicles** and many **Battery Swap Stations**
- A **Pricing Configuration** produces one **Price Quote** per request
- A **Price Quote** produces exactly one **Audit Entry**
- An **A/B Test** maps each **Segment** to a specific **Pricing Configuration**
- An **Event** optionally applies to one **Zone** or one **BSS**
- An **Event Discount** applies zero or more **Price Quotes** during its active window

## Example Dialogue

> **Dev:** "When a **Renter** inserts a battery at a **BSS**, the station calls our pricing API with `vehicle_id`. We then look up the **Vehicle**'s `current_user_id` to find the **Subscriber** and their **Tier**. Is that flow correct?"

> **Domain expert:** "Yes — the **Subscriber**'s **Loyalty Discount** (0.9 for Gold) is applied automatically. The **Price Quote** also checks for any **Active Events** in that **Zone**."

> **Dev:** "What if a **Gold subscriber** swaps a battery at 6 PM in Jakarta Pusat during peak demand?"

> **Domain expert:** "They'd face a **Demand Multiplier** of 1.3 for peak hour, a **Zone Surge Factor** of 1.5 if utilization >80%, but then get 0.9 for **Loyalty Discount** and potentially an **Event Discount** if we're running a promo."

> **Dev:** "So the **Final Price** is: 4,000 × 1.3 × 1.5 × 1.0 × 0.9 × 0.9 = 6,318 Rp for 0.9 kWh?"

> **Domain expert:** "Exactly — and every factor is logged in the **Audit Trail** with an **HMAC Signature** so our **Compliance Officer** can verify no one tampered with the calculation."

## Flagged Ambiguities

- **"duration_hours"** (per test spec) represents **kWh energy consumed** (per business reality). The parameter name implies time, but Electrum's battery swap model charges by energy, not duration. Documented in README with explicit clarification.
- **"user"** is overloaded: means both **Renter** (person renting vehicles) and **API Consumer** (system calling the API). The domain term is **Renter** for the person; **API Consumer** for the calling system.
- **"admin"** is ambiguous between **Operations Admin** (domain role managing config) and the technical **Admin API Key** (auth mechanism). These are related but distinct concepts.
- **"factor" vs "discount" vs "multiplier"**: A **Factor** is the general concept, a **Multiplier** is the numeric value, and a **Discount** is a special case of a Multiplier where the value < 1.0.
- **"A/B test"** in this system means **config routing** (different config per segment), not a single multiplier. This is intentional and documented.

## Pricing Calculation Walkthrough

### The Formula

```
final_price = base_rate × demand × zone_surge × battery × event × loyalty × kWh
```

All factors start at **1.0×** (no effect). Stacking multiplies them together.

### Factor 1: Base Price (4,000 Rp/kWh)

**What it is:** Starting rate before adjustments, covers electricity + battery depreciation + station ops.

**Effect:** `4,000 × kWh = base cost`

---

### Factor 2: Demand Multiplier (Time-of-Day)

**What it is:** Rush hours cost more, quiet hours cost less.

| Time | Multiplier | Rationale |
|---|---|---|
| 5-7 AM weekday | 1.3× | Morning rush, everyone going to work |
| 0-5 AM | 0.9× | Late night, almost no demand |
| Other | 1.0× | Normal |

**Like** Uber surge pricing — balances load across time.

---

### Factor 3: Zone Surge (Fleet Utilization)

**What it is:** Zones with few available vehicles get premium pricing.

| Utilization | Multiplier | Meaning |
|---|---|---|
| >80% | 1.5× | Only 20% of vehicles left |
| 50-80% | 1.2× | Moderately busy |
| <50% | 1.0× | Plenty available |

**Why:** Encourages rentals in quieter zones. Revenue optimization at peak demand.

---

### Factor 4: Battery Discount (Return SoC)

**What it is:** Discount based on estimated battery condition returned to BSS.

**Calculation:**
```
return_soc = (1.8 - kWh_consumed) / 1.8 × 100
```

Assumes all rentals start with 100% charged battery (1.8 kWh max).

| Return SoC | Discount | Why |
|---|---|---|
| ≥60% | 20% off | Excellent — fast-charge ready in 20 min |
| 40-60% | 10% off | Good — still fast-chargeable |
| <40% | None | Depleted — needs 3-6 hour slow charge |

**Incentive:** Return batteries in chargeable state → BSS turns around faster → more bikes available.

---

### Factor 5: Event Discount (Promotions)

**What it is:** Time-limited promos scoped to zone or nationwide.

**Effect:** Admin creates event → applies automatically during its window.

| Example | Multiplier | Trigger |
|---|---|---|
| Ramadan Special (Jakarta Pusat) | 0.8× | Swap in jakarta_pusat during event |
| National Holiday | 0.85× | Any zone during event |
| No event | 1.0× | No effect |

**Why:** Marketing campaigns. Drive traffic to specific zones or times.

---

### Factor 6: Loyalty Discount (Subscription Tier)

**What it is:** Subscribers get automatic discount on every swap.

| Tier | Multiplier | Benefit |
|---|---|---|
| Gold | 0.9× | 10% off every swap |
| Normal | 1.0× | No discount |

**Why:** Reward subscribers, increase retention. Gold members pay monthly → get perks.

---

### Full Example

**Scenario:** Gold subscriber, 6 AM weekday (rush), Jakarta Pusat (96% utilization), returns battery at 50% SoC (0.9 kWh used), no promotions.

```
Step 1: Base         4,000
Step 2: Demand       4,000 × 1.3      = 5,200   (morning rush 1.3×)
Step 3: Zone surge   5,200 × 1.5      = 7,800   (96% util → 1.5×)
Step 4: Battery      7,800 × 0.90     = 7,020   (return SoC 50% → 0.90×)
Step 5: Event        7,020 × 1.0      = 7,020   (no event)
Step 6: Loyalty      7,020 × 0.9      = 6,318   (gold → 0.9×)
Step 7: kWh          6,318 × 0.9      = 5,686   (0.9 kWh consumed)
```

**Final price: Rp 5,686**

---

### Why Multiply (Not Add)?

Adding would give weird results:
- 1.3 + 1.5 + 0.9 + 1.0 + 0.9 = +5.6 (impossible, prices explode)
- Negative discounts break arithmetic

Multiplying means factors **stack naturally**:
- Rush + surge + good battery return = fair combined effect
- Order doesn't matter (commutative)
- Any factor can be disabled by setting to 1.0
