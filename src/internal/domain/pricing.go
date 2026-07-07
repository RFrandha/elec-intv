package domain

import (
	"encoding/json"
	"time"
)

type Vehicle struct {
	ID            string
	Zone          string
	CurrentSOC    float64
	CurrentUserID string
	Model         string
	LastSwap      time.Time
	LastUpdated   time.Time
}

type FleetState struct {
	Zone              string
	TotalVehicles     int
	AvailableVehicles int
	BSSCount          int
	UpdatedAt         time.Time
}

func (f FleetState) Utilization() float64 {
	if f.TotalVehicles == 0 {
		return 0
	}
	return 1.0 - float64(f.AvailableVehicles)/float64(f.TotalVehicles)
}

type User struct {
	ID               string
	SubscriptionTier string
	RentalCount      int
	CreatedAt        time.Time
}

type Tier struct {
	ID                 string
	Name               string
	DiscountMultiplier float64
	Description        string
}

type PricingConfig struct {
	BasePrice            float64                `json:"base_price"`
	DemandRules          []DemandRule           `json:"demand_rules"`
	ZoneSurgeThresholds  []ZoneSurgeThreshold   `json:"zone_surge_thresholds"`
	BatteryDiscountTiers []BatteryDiscountTier  `json:"battery_discount_tiers"`
}

type DemandRule struct {
	DayOfWeek  string  `json:"day_of_week"`
	HourStart  int     `json:"hour_start"`
	HourEnd    int     `json:"hour_end"`
	Multiplier float64 `json:"multiplier"`
}

type ZoneSurgeThreshold struct {
	MinUtilization float64 `json:"min_utilization"`
	Multiplier     float64 `json:"multiplier"`
}

type BatteryDiscountTier struct {
	MaxSOC     float64 `json:"max_soc"`
	Multiplier float64 `json:"multiplier"`
}

type Event struct {
	ID                 int
	Name               string
	Zone               *string
	BSSID              *int
	StartTime          time.Time
	EndTime            time.Time
	DiscountMultiplier float64
	CreatedAt          time.Time
}

func (e Event) IsActive(now time.Time) bool {
	return (now.Equal(e.StartTime) || now.After(e.StartTime)) &&
		(now.Equal(e.EndTime) || now.Before(e.EndTime))
}

type AuditEntry struct {
	QuoteID       string
	VehicleID     string
	UserID        string
	Zone          string
	KWhConsumed   float64
	Inputs        json.RawMessage
	Factors       json.RawMessage
	FinalPrice    float64
	ABSegment     string
	HMACSignature string
	CreatedAt     time.Time
}

type FactorApplied struct {
	Name       string `json:"name"`
	Inputs     any    `json:"inputs"`
	Multiplier float64 `json:"multiplier"`
}

type PricingRequest struct {
	VehicleID       string
	Zone            string
	DurationHours   float64
	ZoneUtilization float64
}

type PricingResult struct {
	QuoteID        string
	VehicleID      string
	Zone           string
	KWhConsumed    float64
	BaseRate       float64
	FinalPrice     float64
	ValidUntil     time.Time
	ABSegment      string
	Factors        []FactorApplied
}

type ConfigHistory struct {
	ConfigID  int
	Version   int
	Config    json.RawMessage
	CreatedAt time.Time
	CreatedBy string
}

type ABTestConfig struct {
	TestID      int
	TestName    string
	SegmentName string
	ConfigID    int
	IsActive    bool
}

type EventInput struct {
	Name               string  `json:"name" binding:"required"`
	Zone               *string `json:"zone"`
	BSSID              *int    `json:"bss_id"`
	StartTime          string  `json:"start_time" binding:"required"`
	EndTime            string  `json:"end_time" binding:"required"`
	DiscountMultiplier float64 `json:"discount_multiplier" binding:"required"`
}

type ConfigInput struct {
	BasePrice            float64              `json:"base_price" binding:"required"`
	DemandRules          []DemandRule         `json:"demand_rules"`
	ZoneSurgeThresholds  []ZoneSurgeThreshold `json:"zone_surge_thresholds"`
	BatteryDiscountTiers []BatteryDiscountTier `json:"battery_discount_tiers"`
}

type ABStats struct {
	Control ABTestStats `json:"control"`
	Variant ABTestStats `json:"variant"`
}

type ABTestStats struct {
	Requests     int     `json:"requests"`
	AvgPrice     float64 `json:"avg_price"`
	TotalRevenue float64 `json:"total_revenue"`
}

type ZoneStats struct {
	Zone      string  `json:"zone"`
	AvgPrice  float64 `json:"avg_price"`
	Count     int     `json:"count"`
}

// Repository interfaces
type VehicleRepository interface {
	FindByID(id string) (*Vehicle, error)
}

type UserRepository interface {
	FindByID(id string) (*User, error)
}

type ConfigRepository interface {
	FindActive() (*PricingConfig, error)
	FindHistory(limit int) ([]ConfigHistory, error)
	Create(input ConfigInput) error
}

type FleetStateRepository interface {
	FindByZone(zone string) (*FleetState, error)
	FindAll() ([]FleetState, error)
}

type EventRepository interface {
	FindActive(now time.Time) ([]Event, error)
	FindAll() ([]Event, error)
	Create(input EventInput) error
	Delete(id int) error
}

type AuditRepository interface {
	Create(entry AuditEntry) error
	FindABStats() (*ABStats, error)
	FindZoneStats(start, end time.Time) ([]ZoneStats, error)
}

type TierRepository interface {
	FindAll() ([]Tier, error)
}
