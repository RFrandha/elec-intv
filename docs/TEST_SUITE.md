# Test Suite

## Quick Start

```bash
go test -v ./src/tests/
```

## Test List

### 1. Pricing Factor Tests

| Test | File | What It Validates |
|------|------|-------------------|
| `TestBatteryDiscount` | `pricing_test.go` | Return SoC thresholds: ≥60%→0.80×, ≥40%→0.90×, <40%→1.0× |
| `TestDemandMultiplier` | `pricing_test.go` | Time-of-day demand rules (weekday 5-7AM→1.3×, 0-5AM→0.9×) |
| `TestZoneSurge` | `pricing_test.go` | Utilization thresholds: >80%→1.5×, 50-80%→1.2×, <50%→1.0× |
| `TestLoyaltyDiscount` | `pricing_test.go` | Gold→0.9×, Normal→1.0× discount application |
| `TestPricingService_Calculate` | `pricing_test.go` | Full integration test scaffolding |

### 2. A/B Test Assignment Tests

| Test | File | What It Validates |
|------|------|-------------------|
| `TestABSegment_Determinism` | `errors_test.go` | `SHA256(user_id) % 100` always returns same segment for same user |
| `TestABSegmentAssignment` | `pricing_test.go` | All users (U001-U010) successfully map to control or variant |

### 3. Validation Tests

| Test | File | What It Validates |
|------|------|-------------------|
| `TestValidation_InvalidZone` | `errors_test.go` | Zone whitelist rejects invalid zones (surabaya, bandung previously), accepts valid ones |
| `TestValidation_KWhBounds` | `errors_test.go` | kWh range enforced: 0.1-1.8 (rejects 0, negatives, >1.8) |
| `TestValidation_EmptyVehicleID` | `errors_test.go` | Empty vehicle_id handled gracefully |

### 4. Config Validation Tests

| Test | File | What It Validates |
|------|------|-------------------|
| `TestConfigValidation` | `errors_test.go` | base_price must be positive and ≤ 100,000 |
| `TestEventValidation` | `errors_test.go` | event discount_multiplier must be 0.01-2.0 |
| `TestTier_DiscountDirection` | `errors_test.go` | Lower multiplier = more discount (Gold 0.9 < Normal 1.0) |

### 5. Zone Tests

| Test | File | What It Validates |
|------|------|-------------------|
| `TestZones_AllPresent` | `errors_test.go` | All 10 zones registered in whitelist; invalid zones rejected |

## Running Specific Tests

```bash
go test -v ./src/tests/ -run TestBatteryDiscount
go test -v ./src/tests/ -run TestABSegment_Determinism
go test -v ./src/tests/ -run TestValidation
```

## Running with Coverage

```bash
go test -coverprofile=coverage.out ./src/tests/
go tool cover -html=coverage.out
```

## Test Architecture

- **14 tests total** across 2 test files
- **Independent expected values:** Tests use manual calculation helpers (e.g., `calcBatteryManual`, `calcDemandManual`) instead of calling the actual service functions — avoids tautological tests
- **Determinism:** A/B segment tests run 10 users twice and verify same segment assignment
- **Edge cases covered:** 0% battery, 100% utilization, >1.8 kWh, empty fields, boundary values (0.1, 1.8)

## Common Test Commands

```bash
# Run all tests
go test -count=1 ./src/tests/

# Run with verbose output
go test -v -count=1 ./src/tests/

# Run integration test (requires database)
go test -v -count=1 ./src/tests/ -run TestPricingService_Calculate

# Check compilation without running
go build ./src/...
```
