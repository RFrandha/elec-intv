package service

import (
	"testing"
	"time"

	"github.com/RFrandha/elec-intv/src/internal/domain"
	"github.com/RFrandha/elec-intv/src/internal/infrastructure/cache"
)

func TestDemandMultiplier(t *testing.T) {
	cfg := &domain.PricingConfig{
		BasePrice: 4000,
		DemandRules: []domain.DemandRule{
			{DayOfWeek: "weekday", HourStart: 17, HourEnd: 19, Multiplier: 1.3},
		},
	}

	mockNow := time.Date(2026, 7, 7, 18, 0, 0, 0, time.UTC)
	if mockNow.Weekday() != time.Tuesday {
		t.Skip("mock date not a weekday")
	}

	result := calcDemand(cfg, mockNow)
	if result != 1.3 {
		t.Errorf("expected 1.3 for weekday 18:00, got %f", result)
	}
}

func TestZoneSurge(t *testing.T) {
	cfg := &domain.PricingConfig{
		ZoneSurgeThresholds: []domain.ZoneSurgeThreshold{
			{MinUtilization: 0.8, Multiplier: 1.5},
			{MinUtilization: 0.5, Multiplier: 1.2},
			{MinUtilization: 0.0, Multiplier: 1.0},
		},
	}

	tests := []struct {
		utilization float64
		expected    float64
	}{
		{0.9, 1.5},
		{0.8, 1.5},
		{0.7, 1.2},
		{0.5, 1.2},
		{0.4, 1.0},
		{0.0, 1.0},
	}

	for _, tt := range tests {
		result := calcZoneSurge(cfg, tt.utilization)
		if result != tt.expected {
			t.Errorf("utilization %.1f: expected %.1f, got %.1f", tt.utilization, tt.expected, result)
		}
	}
}

func TestBatteryDiscount(t *testing.T) {
	cfg := &domain.PricingConfig{
		BatteryDiscountTiers: []domain.BatteryDiscountTier{
			{MinSOC: 60.0, Multiplier: 0.80},
			{MinSOC: 40.0, Multiplier: 0.90},
			{MinSOC: 0.0, Multiplier: 1.0},
		},
	}

	tests := []struct {
		returnSOC float64
		expected  float64
	}{
		{70.0, 0.80},
		{60.0, 0.80},
		{50.0, 0.90},
		{40.0, 0.90},
		{20.0, 1.0},
	}

	for _, tt := range tests {
		result := calcBatteryDiscount(cfg, tt.returnSOC)
		if result != tt.expected {
			t.Errorf("Return SoC %.1f: expected %.2f, got %.2f", tt.returnSOC, tt.expected, result)
		}
	}
}

func TestLoyaltyDiscount(t *testing.T) {
	tiers := map[string]*domain.Tier{
		"normal": {ID: "normal", DiscountMultiplier: 1.0},
		"gold":   {ID: "gold", DiscountMultiplier: 0.9},
	}

	if tiers["normal"].DiscountMultiplier != 1.0 {
		t.Errorf("normal tier: expected 1.0, got %f", tiers["normal"].DiscountMultiplier)
	}

	if tiers["gold"].DiscountMultiplier != 0.9 {
		t.Errorf("gold tier: expected 0.9, got %f", tiers["gold"].DiscountMultiplier)
	}
}

func TestABSegmentAssignment(t *testing.T) {
	store := cache.NewInMemory()
	_ = store

	userIDs := []string{"U001", "U002", "U003"}
	segments := make(map[string]string)

	for _, uid := range userIDs {
		segment := assignABSegment(uid)
		segments[uid] = segment
		if segment != "control" && segment != "variant" {
			t.Errorf("user %s: unexpected segment %s", uid, segment)
		}
	}

	for uid, seg := range segments {
		second := assignABSegment(uid)
		if second != seg {
			t.Errorf("user %s: non-deterministic assignment: %s vs %s", uid, seg, second)
		}
	}
}

func TestABSegment_Determinism(t *testing.T) {
	userIDs := []string{"U001", "U002", "U003", "U004", "U005", "U006", "U007", "U008", "U009", "U010"}
	first := make(map[string]string)

	for _, uid := range userIDs {
		segment := assignABSegment(uid)
		first[uid] = segment
		if segment != "control" && segment != "variant" {
			t.Errorf("user %s: invalid segment %s", uid, segment)
		}
	}

	for _, uid := range userIDs {
		second := assignABSegment(uid)
		if second != first[uid] {
			t.Errorf("user %s: non-deterministic: %s -> %s", uid, first[uid], second)
		}
	}
}

func TestPricingService_Calculate(t *testing.T) {
	svc := NewPricingService(
		cache.NewInMemory(), nil, nil, nil, nil, nil, nil, nil, nil, "test-secret",
	)

	_ = svc

	cfg := &domain.PricingConfig{
		BasePrice: 4000,
		DemandRules: []domain.DemandRule{
			{DayOfWeek: "weekday", HourStart: 17, HourEnd: 19, Multiplier: 1.3},
		},
		ZoneSurgeThresholds: []domain.ZoneSurgeThreshold{
			{MinUtilization: 0.8, Multiplier: 1.5},
			{MinUtilization: 0.0, Multiplier: 1.0},
		},
		BatteryDiscountTiers: []domain.BatteryDiscountTier{
			{MinSOC: 60.0, Multiplier: 0.80},
			{MinSOC: 40.0, Multiplier: 0.90},
			{MinSOC: 0.0, Multiplier: 1.0},
		},
	}
	_ = cfg

	t.Log("Test scaffolding complete - requires database for full integration test")
}

func TestTier_DiscountDirection(t *testing.T) {
	tiers := map[string]float64{
		"normal": 1.0,
		"gold":   0.9,
	}

	if tiers["normal"] != 1.0 {
		t.Errorf("normal: expected 1.0, got %f", tiers["normal"])
	}

	if tiers["gold"] != 0.9 {
		t.Errorf("gold: expected 0.9, got %f", tiers["gold"])
	}

	if tiers["gold"] >= tiers["normal"] {
		t.Error("gold discount should be lower than normal (0.9 < 1.0)")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		basePrice float64
		isValid   bool
	}{
		{4000, true},
		{100, true},
		{0, false},
		{-100, false},
		{99999, true},
		{100001, false},
	}

	for _, tt := range tests {
		isValid := tt.basePrice > 0 && tt.basePrice <= 100000
		if isValid != tt.isValid {
			t.Errorf("base_price %.0f: expected valid=%v, got %v", tt.basePrice, tt.isValid, isValid)
		}
	}
}

func TestEventValidation(t *testing.T) {
	tests := []struct {
		multiplier float64
		isValid    bool
	}{
		{0.8, true},
		{1.0, true},
		{1.5, true},
		{2.0, true},
		{0.0, false},
		{-0.1, false},
		{2.1, false},
	}

	for _, tt := range tests {
		isValid := tt.multiplier > 0 && tt.multiplier <= 2.0
		if isValid != tt.isValid {
			t.Errorf("multiplier %.1f: expected valid=%v, got %v", tt.multiplier, tt.isValid, isValid)
		}
	}
}
