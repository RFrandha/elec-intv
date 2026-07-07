package dto

import "github.com/RFrandha/elec-intv/src/internal/domain"

type PricingRequest struct {
	VehicleID       string  `json:"vehicle_id" binding:"required"`
	Zone            string  `json:"zone" binding:"required"`
	DurationHours   float64 `json:"duration_hours" binding:"required"`
	ZoneUtilization float64 `json:"zone_utilization"`
}

type PricingResponse struct {
	QuoteID        string                 `json:"quote_id"`
	VehicleID      string                 `json:"vehicle_id"`
	Zone           string                 `json:"zone"`
	KWhConsumed    float64                `json:"kwh_consumed"`
	BaseRatePerKWh float64                `json:"base_rate_per_kwh"`
	FinalPrice     float64                `json:"final_price"`
	ValidUntil     string                 `json:"valid_until"`
	ABSegment      string                 `json:"ab_segment"`
}

func NewPricingResponse(result *domain.PricingResult) PricingResponse {
	return PricingResponse{
		QuoteID:        result.QuoteID,
		VehicleID:      result.VehicleID,
		Zone:           result.Zone,
		KWhConsumed:    result.KWhConsumed,
		BaseRatePerKWh: result.BaseRate,
		FinalPrice:     result.FinalPrice,
		ValidUntil:     result.ValidUntil.UTC().Format("2006-01-02T15:04:05Z"),
		ABSegment:      result.ABSegment,
	}
}

type BreakdownResponse struct {
	QuoteID        string `json:"quote_id"`
	BaseRatePerKWh float64 `json:"base_rate_per_kwh"`
	KWhConsumed    float64 `json:"kwh_consumed"`
	Factors        []FactorDTO `json:"factors"`
	FinalPrice     float64 `json:"final_price"`
}

type FactorDTO struct {
	Name       string `json:"name"`
	Inputs     any    `json:"inputs"`
	Multiplier float64 `json:"multiplier"`
}

type ConfigResponse struct {
	BasePrice            float64                    `json:"base_price"`
	DemandRules          []domain.DemandRule        `json:"demand_rules"`
	ZoneSurgeThresholds  []domain.ZoneSurgeThreshold `json:"zone_surge_thresholds"`
	BatteryDiscountTiers []domain.BatteryDiscountTier `json:"battery_discount_tiers"`
}

type EventResponse struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Zone        *string   `json:"zone"`
	BSSID       *int      `json:"bss_id"`
	StartTime   string    `json:"start_time"`
	EndTime     string    `json:"end_time"`
	Multiplier  float64   `json:"discount_multiplier"`
	IsActive    bool      `json:"is_active"`
}

type FleetStateResponse struct {
	Zone              string  `json:"zone"`
	TotalVehicles     int     `json:"total_vehicles"`
	AvailableVehicles int     `json:"available_vehicles"`
	UtilizationPct    float64 `json:"utilization_pct"`
	BSSCount          int     `json:"bss_count"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ConfigHistoryResponse struct {
	ConfigID  int    `json:"config_id"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
}

type ABStatsResponse struct {
	Control SegmentStats `json:"control"`
	Variant SegmentStats `json:"variant"`
}

type SegmentStats struct {
	Requests     int     `json:"requests"`
	AvgPrice     float64 `json:"avg_price"`
	TotalRevenue float64 `json:"total_revenue"`
}
