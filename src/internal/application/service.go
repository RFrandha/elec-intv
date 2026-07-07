package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/RFrandha/elec-intv/src/internal/domain"
	"github.com/RFrandha/elec-intv/src/internal/infrastructure/cache"
	"github.com/RFrandha/elec-intv/src/internal/infrastructure/database"
)

type PricingService struct {
	cache      *cache.InMemory
	vehicleRepo *database.VehicleRepo
	userRepo    *database.UserRepo
	fleetRepo   *database.FleetStateRepo
	eventRepo   *database.EventRepo
	auditRepo   *database.AuditRepo
	configRepo  *database.ConfigRepo
	tierRepo    *database.TierRepo
	hmacSecret  string
	cacheMu     sync.Mutex
}

func NewPricingService(
	cache *cache.InMemory,
	vehicleRepo *database.VehicleRepo,
	userRepo *database.UserRepo,
	fleetRepo *database.FleetStateRepo,
	eventRepo *database.EventRepo,
	auditRepo *database.AuditRepo,
	configRepo *database.ConfigRepo,
	tierRepo *database.TierRepo,
	hmacSecret string,
) *PricingService {
	return &PricingService{
		cache:       cache,
		vehicleRepo: vehicleRepo,
		userRepo:    userRepo,
		fleetRepo:   fleetRepo,
		eventRepo:   eventRepo,
		auditRepo:   auditRepo,
		configRepo:  configRepo,
		tierRepo:    tierRepo,
		hmacSecret:  hmacSecret,
	}
}

func (s *PricingService) StartCacheUpdater() {
	go func() {
		ticker30 := time.NewTicker(30 * time.Second)
		for range ticker30.C {
			s.refreshConfig()
			s.refreshEvents()
		}
	}()

	go func() {
		ticker5m := time.NewTicker(5 * time.Minute)
		for range ticker5m.C {
			s.refreshTiers()
		}
	}()

	log.Println("Cache updater started (config/events: 30s, tiers: 5m)")
}

func (s *PricingService) RefreshNow() {
	s.refreshConfig()
	s.refreshEvents()
	s.refreshTiers()
	log.Println("Cache refreshed manually")
}

func (s *PricingService) refreshConfig() {
	config, err := s.configRepo.FindActive()
	if err != nil {
		log.Printf("Failed to load config: %v", err)
		return
	}
	s.cache.SetConfig(config)
	log.Printf("Config refreshed: base=%f", config.BasePrice)
}

func (s *PricingService) refreshEvents() {
	events, err := s.eventRepo.FindActive(time.Now())
	if err != nil {
		log.Printf("Failed to load events: %v", err)
		return
	}
	s.cache.SetEvents(events)
}

func (s *PricingService) refreshTiers() {
	tiers, err := s.tierRepo.FindAll()
	if err != nil {
		log.Printf("Failed to load tiers: %v", err)
		return
	}
	s.cache.SetTiers(tiers)
}

func (s *PricingService) Calculate(req domain.PricingRequest) (*domain.PricingResult, error) {
	// 1. Get vehicle
	vehicle, err := s.vehicleRepo.FindByID(req.VehicleID)
	if err != nil {
		return nil, err
	}

	// 2. Get user for loyalty
	user, err := s.userRepo.FindByID(vehicle.CurrentUserID)
	if err != nil {
		user = &domain.User{SubscriptionTier: "normal"}
	}

	// 3. Get fleet state for zone utilization
	fleetState, err := s.fleetRepo.FindByZone(req.Zone)
	if err == nil {
		req.ZoneUtilization = fleetState.Utilization()
	}

	// 4. Calculate factors
	config := s.cache.GetConfig()
	tiers := s.cache.GetTiers()
	events := s.cache.GetEvents()

	factors := []domain.FactorApplied{}
	multiplier := 1.0

	// Demand multiplier
	demandMult := calcDemand(config, time.Now())
	factors = append(factors, domain.FactorApplied{Name: "demand_multiplier", Inputs: map[string]any{"time": time.Now().Format("15:04 Mon")}, Multiplier: demandMult})
	multiplier *= demandMult

	// Zone surge
	zoneMult := calcZoneSurge(config, req.ZoneUtilization)
	factors = append(factors, domain.FactorApplied{Name: "zone_surge", Inputs: map[string]any{"zone": req.Zone, "utilization": req.ZoneUtilization}, Multiplier: zoneMult})
	multiplier *= zoneMult

	// Battery discount
	batteryMult := calcBatteryDiscount(config, vehicle.CurrentSOC)
	factors = append(factors, domain.FactorApplied{Name: "battery_discount", Inputs: map[string]any{"soc": vehicle.CurrentSOC}, Multiplier: batteryMult})
	multiplier *= batteryMult

	// Event discount
	eventMult := calcEventDiscount(events, req.Zone)
	factors = append(factors, domain.FactorApplied{Name: "event_discount", Inputs: map[string]any{"events_active": len(events)}, Multiplier: eventMult})
	multiplier *= eventMult

	// Loyalty discount
	loyaltyMult := calcLoyaltyDiscount(tiers, user.SubscriptionTier)
	factors = append(factors, domain.FactorApplied{Name: "loyalty_discount", Inputs: map[string]any{"tier": user.SubscriptionTier}, Multiplier: loyaltyMult})
	multiplier *= loyaltyMult

	// A/B segment
	abSegment := assignABSegment(user.ID)
	_ = abSegment // config routing not yet implemented in v1

	finalPrice := config.BasePrice * multiplier * req.DurationHours

	quoteID := fmt.Sprintf("Q-%s", uuid.New().String()[:8])

	inputsJSON, _ := json.Marshal(req)
	factorsJSON, _ := json.Marshal(factors)

	audit := domain.AuditEntry{
		QuoteID:     quoteID,
		VehicleID:   req.VehicleID,
		UserID:      user.ID,
		Zone:        req.Zone,
		KWhConsumed: req.DurationHours,
		Inputs:      inputsJSON,
		Factors:     factorsJSON,
		FinalPrice:  finalPrice,
		ABSegment:   abSegment,
		CreatedAt:   time.Now(),
	}

	audit.HMACSignature = s.signAuditEntry(audit)

	if err := s.auditRepo.Create(audit); err != nil {
		log.Printf("Failed to write audit entry: %v", err)
	}

	return &domain.PricingResult{
		QuoteID:    quoteID,
		VehicleID:  req.VehicleID,
		Zone:       req.Zone,
		KWhConsumed: req.DurationHours,
		BaseRate:   config.BasePrice,
		FinalPrice: finalPrice,
		ValidUntil: time.Now().Add(30 * time.Second),
		ABSegment:  abSegment,
		Factors:    factors,
	}, nil
}

func (s *PricingService) VerifyAudit(entry domain.AuditEntry) bool {
	expected := s.signAuditEntry(entry)
	return hmac.Equal([]byte(expected), []byte(entry.HMACSignature))
}

func (s *PricingService) signAuditEntry(entry domain.AuditEntry) string {
	data := fmt.Sprintf("%s%s%s%s%f%s",
		entry.QuoteID, entry.UserID, entry.VehicleID,
		entry.Zone, entry.FinalPrice, entry.CreatedAt.Format(time.RFC3339))

	mac := hmac.New(sha256.New, []byte(s.hmacSecret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func calcDemand(config *domain.PricingConfig, now time.Time) float64 {
	hour := now.Hour()
	weekday := now.Weekday() != time.Saturday && now.Weekday() != time.Sunday

	for _, rule := range config.DemandRules {
		if (rule.DayOfWeek == "weekday" && weekday) || (rule.DayOfWeek == "weekend" && !weekday) {
			if hour >= rule.HourStart && hour < rule.HourEnd {
				return rule.Multiplier
			}
		}
	}

	return 1.0
}

func calcZoneSurge(config *domain.PricingConfig, utilization float64) float64 {
	for _, t := range config.ZoneSurgeThresholds {
		if utilization >= t.MinUtilization {
			return t.Multiplier
		}
	}
	return 1.0
}

func calcBatteryDiscount(config *domain.PricingConfig, soc float64) float64 {
	for _, t := range config.BatteryDiscountTiers {
		if soc <= t.MaxSOC {
			return t.Multiplier
		}
	}
	return 1.0
}

func calcEventDiscount(events []domain.Event, zone string) float64 {
	now := time.Now()
	for _, e := range events {
		if !e.IsActive(now) {
			continue
		}
		if e.Zone == nil || *e.Zone == zone {
			return e.DiscountMultiplier
		}
	}
	return 1.0
}

func calcLoyaltyDiscount(tiers map[string]*domain.Tier, tierID string) float64 {
	if t, ok := tiers[tierID]; ok {
		return t.DiscountMultiplier
	}
	return 1.0
}

func assignABSegment(userID string) string {
	h := sha256.Sum256([]byte(userID))
	val := int(h[0])%100 + int(h[1])%100
	if val%100 < 50 {
		return "control"
	}
	return "variant"
}
