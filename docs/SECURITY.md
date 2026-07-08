# Security Considerations

## API Key Authentication

### Two-Tier Key System

| Tier | Key | Capabilities | Rate Limit |
|------|-----|-------------|-----------|
| **Read-only Key** | `<READ_ONLY_API_KEY>` | Pricing queries, breakdown | 100 req/min per IP |
| **Admin Key** | `<ADMIN_API_KEY>` | Config management, events, fleet | 10 req/min reads, 1 req/min writes |

**Note:** API keys provided separately in onboarding PDF (not in repository).

### How It Works

- Keys passed via `X-API-Key` HTTP header
- Public endpoints require read-only OR admin key
- Admin endpoints require admin key specifically
- Middleware validates key on every request (constant-time comparison to prevent timing attacks)

### Key Management (Production Improvements)

Currently stored as environment variables (for 72h test only):
```bash
# docker-compose.yml or Cloud Run env vars
ADMIN_API_KEY=<your-admin-key>
READ_ONLY_API_KEY=<your-read-only-key>
```

**Production improvements:**
- Use GCP Secret Manager for key storage (no env vars)
- Rotate keys periodically (Secret Manager versioning)
- Audit key usage in Cloud Logging
- Separate keys per environment (dev/staging/production)

## Input Validation

### Parameter Validation

| Parameter | Validation Rule |
|-----------|----------------|
| `vehicle_id` | Must match format `V[0-9]+` (e.g., V001) |
| `zone` | Must be one of 10 allowed zones (whitelist) |
| `duration_hours` | Float between 0.1 and 1.8 (max battery capacity) |

### Zone Whitelist

Only 10 zones accepted. Any other zone returns 400 Bad Request:
```
jakarta_pusat, jakarta_selatan, jakarta_barat,
jakarta_timur, jakarta_utara, bogor,
depok, tangerang, bekasi, bandung
```

### Request Size Limits

- Admin PUT /config: max 10KB JSON payload
- Admin POST /events: max 5KB JSON payload
- Pricing query parameters: max 500 characters

### SQL Injection Prevention

All database queries use parameterized statements (`$1`, `$2`, etc.):
```go
// Safe: parameterized
db.QueryRow("SELECT * FROM pricing.vehicles WHERE vehicle_id = $1", vehicleID)

// NOT USED: string interpolation (unsafe)
// db.QueryRow("SELECT * FROM pricing.vehicles WHERE vehicle_id = '" + vehicleID + "'")
```

## Secret Management

### Current Approach

| Secret | Stored As | Access |
|--------|-----------|--------|
| HMAC Secret | Environment variable | Code reads at startup |
| API Keys | Environment variables | Middleware validates |
| Database URL | Environment variable | Code connects at startup |

### Security: What Environment Variables DON'T Solve

- Env vars can leak in error logs (we DON'T print secrets in logs)
- Env vars visible in Cloud Run console (limited to project admins)
- Env vars visible in Docker Compose (local dev only)

**For 72h test:** Acceptable. Keys are demo keys, not production secrets.

**Production improvements (documented in SECURITY.md):**
- Use GCP Secret Manager for all secrets
- IAM roles to restrict secret access
- Audit secret access in Cloud Audit Logs

## Rate Limiting

### Rate Limit Implementation

Simple in-memory rate limiter (token bucket per IP):

| Endpoint Type | Limit | Window | Response |
|---------------|-------|--------|----------|
| Public pricing | 100 requests | Per minute | 429 Too Many Requests |
| Admin reads | 10 requests | Per minute | 429 Too Many Requests |
| Admin writes | 1 request | Per minute | 429 Too Many Requests |

### Rate Limit Caveats

- **Per-instance** (not shared across Cloud Run instances)
- Resets when instance restarts (acceptable for 72h test)
- In production: use Redis or Cloud Armor for distributed rate limiting

## HMAC Audit Trail

### Tamper Detection

Every audit entry signed with:
```
signature = HMAC-SHA256(quote_id + user_id + vehicle_id + zone + final_price + created_at, secret)
```

**Why HMAC:**
- Symmetric: same key signs and verifies
- Lightweight: <5μs per calculation
- Tamper-evident: changing any field invalidates signature

**Verification workflow:**
1. Compliance officer retrieves audit entry
2. Recomputes HMAC with known secret
3. Compares with stored signature
4. If mismatch → entry tampered

### Append-Only Invariant

- `pricing_audit` table has NO UPDATE or DELETE triggers
- Application code never issues UPDATE/DELETE on audit table
- Database user for service account has INSERT-only permission on audit table

## Input Validation Edge Cases

### What Happens When...

| Attack Attempt | Result |
|---------------|--------|
| SQL injection (`' OR 1=1--`) | Parameterized query rejects |
| Non-existent vehicle (`V999`) | `{"error":"vehicle not found"}` |
| Invalid zone (`xyz123`) | `{"error":"invalid zone"}` |
| Negative kWh (`-1.0`) | `{"error":"invalid duration_hours: must be 0.1-1.8"}` |
| Zero kWh (`0`) | `{"error":"invalid duration_hours: must be 0.1-1.8"}` |
| Overflow kWh (`9999`) | `{"error":"invalid duration_hours: must be 0.1-1.8"}` |
| Missing API key | `{"error":"unauthorized"}` |
| Invalid API key | `{"error":"invalid API key"}` |
| Rate limit exceeded | `{"error":"rate limit exceeded"}` |
| Admin endpoint with read key | `{"error":"unauthorized, admin key required"}` |
| CORS preflight (OPTIONS) | 200 OK (no auth required) |

## TLS/HTTPS

- Cloud Run automatically provisions TLS certificates (Let's Encrypt)
- All traffic is HTTPS-only (HTTP redirects to HTTPS)
- Locally via Docker Compose: HTTP (no TLS, for demo purposes only)

## Dependency Security

### Go Dependencies

```
github.com/lib/pq          — PostgreSQL driver (widely used, maintained)
github.com/google/uuid     — UUID generation (by Google, lightweight)
```

- No third-party web framework (minimal attack surface)
- No vulnerable dependencies (verified via `go vet`)

### Container Security

- **Dockerfile uses distroless base image** (alpine:latest)
- No shell access in runtime container
- Only Go binary + CA certificates in final image
- No package managers or compilers at runtime

## Security Compliance Checklist

- [x] API key authentication (two tiers)
- [x] Input validation (zone whitelist, bounds checking)
- [x] SQL injection prevention (parameterized queries)
- [x] Rate limiting (per IP/endpoint)
- [x] TLS/HTTPS (Cloud Run managed)
- [x] Audit trail (append-only, HMAC-signed)
- [x] Minimal dependencies (secure supply chain)
- [x] No secret leaks in code (env vars only)
- [x] Container security (distroless image)
- [x] CORS protection (no reflection)
