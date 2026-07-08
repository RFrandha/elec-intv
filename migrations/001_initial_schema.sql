-- Tiers
CREATE TABLE IF NOT EXISTS pricing.tiers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    discount_multiplier NUMERIC(5,2) NOT NULL DEFAULT 1.0,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Users
CREATE TABLE IF NOT EXISTS pricing.users (
    user_id TEXT PRIMARY KEY,
    subscription_tier_id TEXT NOT NULL REFERENCES pricing.tiers(id),
    rental_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Vehicles
CREATE TABLE IF NOT EXISTS pricing.vehicles (
    vehicle_id TEXT PRIMARY KEY,
    zone TEXT NOT NULL,
    current_soc NUMERIC(5,2),
    current_user_id TEXT REFERENCES pricing.users(user_id),
    model TEXT,
    last_swap_timestamp TIMESTAMPTZ,
    last_updated TIMESTAMPTZ DEFAULT NOW()
);

-- Fleet State
CREATE TABLE IF NOT EXISTS pricing.fleet_state (
    zone TEXT PRIMARY KEY,
    total_vehicles INT NOT NULL DEFAULT 0,
    available_vehicles INT NOT NULL DEFAULT 0,
    bss_count INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Pricing Configs
CREATE TABLE IF NOT EXISTS pricing.pricing_configs (
    config_id SERIAL PRIMARY KEY,
    version INT NOT NULL UNIQUE,
    config_jsonb JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by TEXT DEFAULT 'system'
);

-- Active Config
CREATE TABLE IF NOT EXISTS pricing.active_config (
    id INT PRIMARY KEY DEFAULT 1,
    config_id INT NOT NULL REFERENCES pricing.pricing_configs(config_id),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Events
CREATE TABLE IF NOT EXISTS pricing.events (
    event_id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    zone TEXT,
    bss_id INT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    discount_multiplier NUMERIC(5,2) NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- A/B Test Configs
CREATE TABLE IF NOT EXISTS pricing.ab_test_configs (
    test_id SERIAL PRIMARY KEY,
    test_name TEXT NOT NULL,
    segment_name TEXT NOT NULL,
    config_id INT NOT NULL REFERENCES pricing.pricing_configs(config_id),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Pricing Audit Trail
CREATE TABLE IF NOT EXISTS pricing.pricing_audit (
    quote_id UUID PRIMARY KEY,
    vehicle_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    zone TEXT NOT NULL,
    kwh_consumed NUMERIC(5,2) NOT NULL,
    inputs JSONB,
    factors_applied JSONB,
    final_price NUMERIC(12,2) NOT NULL,
    ab_segment TEXT NOT NULL,
    hmac_signature TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_pricing_audit_user_id_created_at 
ON pricing.pricing_audit(user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_pricing_audit_created_at 
ON pricing.pricing_audit(created_at);

CREATE INDEX IF NOT EXISTS idx_pricing_audit_ab_segment 
ON pricing.pricing_audit(ab_segment);

CREATE INDEX IF NOT EXISTS idx_events_time_range 
ON pricing.events(start_time, end_time);

CREATE INDEX IF NOT EXISTS idx_fleet_state_zone 
ON pricing.fleet_state(zone);

-- Insert default tiers
INSERT INTO pricing.tiers (id, name, discount_multiplier, description) VALUES
  ('normal', 'Normal', 1.0, 'Standard pricing - no subscriber discount'),
  ('gold', 'Gold', 0.9, 'Subscriber tier - 10% discount on all swaps')
ON CONFLICT (id) DO NOTHING;

-- Insert default config
INSERT INTO pricing.pricing_configs (version, config_jsonb, created_by) VALUES
  (1, '{"base_price": 4000, "demand_rules": [{"day_of_week": "weekday", "hour_start": 5, "hour_end": 7, "multiplier": 1.3}, {"day_of_week": "weekday", "hour_start": 0, "hour_end": 5, "multiplier": 0.9}], "zone_surge_thresholds": [{"min_utilization": 0.8, "multiplier": 1.5}, {"min_utilization": 0.5, "multiplier": 1.2}, {"min_utilization": 0.0, "multiplier": 1.0}], "battery_discount_tiers": [{"min_soc": 60.0, "multiplier": 0.80}, {"min_soc": 40.0, "multiplier": 0.90}, {"min_soc": 0.0, "multiplier": 1.0}]}'::jsonb, 'system')
ON CONFLICT (version) DO UPDATE SET config_jsonb = EXCLUDED.config_jsonb;

-- Set active config
INSERT INTO pricing.active_config (id, config_id) VALUES (1, 1)
ON CONFLICT (id) DO UPDATE SET config_id = 1;

-- A/B Test Configs
CREATE TABLE IF NOT EXISTS pricing.ab_test_configs (
    test_id SERIAL PRIMARY KEY,
    test_name TEXT NOT NULL DEFAULT 'default',
    segment_name TEXT NOT NULL,
    config_id INT NOT NULL REFERENCES pricing.pricing_configs(config_id),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(test_name, segment_name)
);

CREATE INDEX IF NOT EXISTS idx_ab_test_active
ON pricing.ab_test_configs(is_active);

-- Insert variant config (version 2) if not exists
INSERT INTO pricing.pricing_configs (version, config_jsonb, created_by) VALUES
  (2, '{"base_price": 4500, "demand_rules": [{"day_of_week": "weekday", "hour_start": 5, "hour_end": 7, "multiplier": 1.3}, {"day_of_week": "weekday", "hour_start": 0, "hour_end": 5, "multiplier": 0.9}], "zone_surge_thresholds": [{"min_utilization": 0.8, "multiplier": 1.5}, {"min_utilization": 0.5, "multiplier": 1.2}, {"min_utilization": 0.0, "multiplier": 1.0}], "battery_discount_tiers": [{"min_soc": 60.0, "multiplier": 0.80}, {"min_soc": 40.0, "multiplier": 0.90}, {"min_soc": 0.0, "multiplier": 1.0}]}'::jsonb, 'system')
ON CONFLICT (version) DO UPDATE SET config_jsonb = EXCLUDED.config_jsonb;

-- Seed A/B test mapping: control=version1, variant=version2
INSERT INTO pricing.ab_test_configs (test_name, segment_name, config_id, is_active)
SELECT 'default', 'control', (SELECT config_id FROM pricing.pricing_configs WHERE version = 1 LIMIT 1), true
WHERE NOT EXISTS (SELECT 1 FROM pricing.ab_test_configs WHERE test_name = 'default' AND segment_name = 'control')
UNION ALL
SELECT 'default', 'variant', (SELECT config_id FROM pricing.pricing_configs WHERE version = 2 LIMIT 1), true
WHERE NOT EXISTS (SELECT 1 FROM pricing.ab_test_configs WHERE test_name = 'default' AND segment_name = 'variant');
