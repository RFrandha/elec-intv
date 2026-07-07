package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/RFrandha/elec-intv/src/internal/domain"
)

type ConfigRepo struct {
	db *DB
}

func NewConfigRepo(db *DB) *ConfigRepo {
	return &ConfigRepo{db: db}
}

func (r *ConfigRepo) FindActive() (*domain.PricingConfig, error) {
	var configJSON string
	err := r.db.QueryRow(
		`SELECT pc.config_jsonb::text
		 FROM pricing.active_config ac
		 JOIN pricing.pricing_configs pc ON pc.config_id = ac.config_id
		 WHERE ac.id = 1`,
	).Scan(&configJSON)

	if err == sql.ErrNoRows {
		return &domain.PricingConfig{BasePrice: 4000}, nil
	}
	if err != nil {
		return nil, err
	}

	var config domain.PricingConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ConfigRepo) FindHistory(limit int) ([]domain.ConfigHistory, error) {
	rows, err := r.db.Query(
		`SELECT config_id, version, config_jsonb::text, created_at, created_by
		 FROM pricing.pricing_configs ORDER BY version DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []domain.ConfigHistory
	for rows.Next() {
		var c domain.ConfigHistory
		var configText string
		if err := rows.Scan(&c.ConfigID, &c.Version, &configText, &c.CreatedAt, &c.CreatedBy); err != nil {
			continue
		}
		c.Config = json.RawMessage(configText)
		configs = append(configs, c)
	}
	return configs, nil
}

func (r *ConfigRepo) Create(input domain.ConfigInput) error {
	configJSON, err := json.Marshal(input)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(
		`INSERT INTO pricing.pricing_configs (version, config_jsonb, created_by)
		 VALUES ((SELECT COALESCE(MAX(version), 0) + 1 FROM pricing.pricing_configs), $1::jsonb, 'admin')`,
		string(configJSON),
	)
	return err
}

type EventRepo struct {
	db *DB
}

func NewEventRepo(db *DB) *EventRepo {
	return &EventRepo{db: db}
}

func (r *EventRepo) FindActive(now time.Time) ([]domain.Event, error) {
	rows, err := r.db.Query(
		`SELECT event_id, name, zone, bss_id, start_time, end_time, discount_multiplier, created_at
		 FROM pricing.events WHERE end_time > $1 ORDER BY start_time`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var e domain.Event
		if err := rows.Scan(&e.ID, &e.Name, &e.Zone, &e.BSSID, &e.StartTime, &e.EndTime, &e.DiscountMultiplier, &e.CreatedAt); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *EventRepo) FindAll() ([]domain.Event, error) {
	rows, err := r.db.Query(
		`SELECT event_id, name, zone, bss_id, start_time, end_time, discount_multiplier, created_at
		 FROM pricing.events ORDER BY start_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var e domain.Event
		if err := rows.Scan(&e.ID, &e.Name, &e.Zone, &e.BSSID, &e.StartTime, &e.EndTime, &e.DiscountMultiplier, &e.CreatedAt); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *EventRepo) Create(input domain.EventInput) (int, error) {
	start, err := time.Parse(time.RFC3339, input.StartTime)
	if err != nil {
		return 0, fmt.Errorf("invalid start_time: %w", err)
	}

	end, err := time.Parse(time.RFC3339, input.EndTime)
	if err != nil {
		return 0, fmt.Errorf("invalid end_time: %w", err)
	}

	var eventID int
	err = r.db.QueryRow(
		`INSERT INTO pricing.events (name, zone, bss_id, start_time, end_time, discount_multiplier)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING event_id`,
		input.Name, input.Zone, input.BSSID, start, end, input.DiscountMultiplier,
	).Scan(&eventID)

	return eventID, err
}

func (r *EventRepo) Delete(id int) error {
	result, err := r.db.Exec(`DELETE FROM pricing.events WHERE event_id = $1`, id)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("event not found")
	}
	return nil
}

type AuditRepo struct {
	db *DB
}

func NewAuditRepo(db *DB) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Create(entry domain.AuditEntry) error {
	_, err := r.db.Exec(
		`INSERT INTO pricing.pricing_audit
		 (quote_id, vehicle_id, user_id, zone, kwh_consumed, inputs, factors_applied, final_price, ab_segment, hmac_signature)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		entry.QuoteID, entry.VehicleID, entry.UserID, entry.Zone, entry.KWhConsumed,
		entry.Inputs, entry.Factors, entry.FinalPrice, entry.ABSegment, entry.HMACSignature,
	)
	return err
}

func (r *AuditRepo) FindABStats() (*domain.ABStats, error) {
	stats := &domain.ABStats{}

	stats.Control.Requests, stats.Control.AvgPrice, stats.Control.TotalRevenue = r.getSegmentStats("control")
	stats.Variant.Requests, stats.Variant.AvgPrice, stats.Variant.TotalRevenue = r.getSegmentStats("variant")

	return stats, nil
}

func (r *AuditRepo) getSegmentStats(segment string) (int, float64, float64) {
	var count int
	var avgPrice, totalRevenue float64

	err := r.db.QueryRow(
		`SELECT COUNT(*), COALESCE(AVG(final_price), 0), COALESCE(SUM(final_price), 0)
		 FROM pricing.pricing_audit WHERE ab_segment = $1`, segment,
	).Scan(&count, &avgPrice, &totalRevenue)

	if err != nil {
		return 0, 0, 0
	}
	return count, avgPrice, totalRevenue
}

func (r *AuditRepo) FindZoneStats(start, end time.Time) ([]domain.ZoneStats, error) {
	rows, err := r.db.Query(
		`SELECT zone, AVG(final_price), COUNT(*)
		 FROM pricing.pricing_audit WHERE created_at BETWEEN $1 AND $2
		 GROUP BY zone ORDER BY zone`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []domain.ZoneStats
	for rows.Next() {
		var s domain.ZoneStats
		if err := rows.Scan(&s.Zone, &s.AvgPrice, &s.Count); err != nil {
			continue
		}
		stats = append(stats, s)
	}
	return stats, nil
}

type TierRepo struct {
	db *DB
}

func NewTierRepo(db *DB) *TierRepo {
	return &TierRepo{db: db}
}

func (r *TierRepo) FindAll() ([]domain.Tier, error) {
	rows, err := r.db.Query(`SELECT id, name, discount_multiplier, description FROM pricing.tiers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tiers []domain.Tier
	for rows.Next() {
		var t domain.Tier
		if err := rows.Scan(&t.ID, &t.Name, &t.DiscountMultiplier, &t.Description); err != nil {
			continue
		}
		tiers = append(tiers, t)
	}
	return tiers, nil
}
