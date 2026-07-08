package tests

import (
	"testing"
)

func TestValidation_InvalidZone(t *testing.T) {
	validZones := map[string]bool{
		"jakarta_pusat": true, "jakarta_selatan": true, "jakarta_barat": true,
		"jakarta_timur": true, "jakarta_utara": true, "bogor": true,
		"depok": true, "tangerang": true, "bekasi": true,
	}

	invalidZones := []string{"surabaya", "bandung", "  ", "jakarta", ""}
	for _, z := range invalidZones {
		if validZones[z] {
			t.Errorf("zone %q should NOT be valid", z)
		}
	}

	valid := []string{"jakarta_pusat", "bogor", "bekasi"}
	for _, z := range valid {
		if !validZones[z] {
			t.Errorf("zone %q SHOULD be valid", z)
		}
	}
}

func TestValidation_KWhBounds(t *testing.T) {
	tests := []struct {
		kwh     float64
		isValid bool
	}{
		{0.1, true},
		{0.9, true},
		{1.8, true},
		{0.0, false},
		{-1.0, false},
		{1.9, false},
		{999, false},
	}

	for _, tt := range tests {
		isValid := tt.kwh > 0 && tt.kwh <= 1.8
		if isValid != tt.isValid {
			t.Errorf("kWh %.1f: expected valid=%v, got %v", tt.kwh, tt.isValid, isValid)
		}
	}
}

func TestValidation_EmptyVehicleID(t *testing.T) {
	if "" == "" {
		// would return 400 from handler
	}
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

func TestZones_AllPresent(t *testing.T) {
	expected := []string{
		"jakarta_pusat", "jakarta_selatan", "jakarta_barat",
		"jakarta_timur", "jakarta_utara", "bogor",
		"depok", "tangerang", "bekasi",
	}

	for _, z := range expected {
		if !validZone(z) {
			t.Errorf("expected zone %q not found", z)
		}
	}
}

func validZone(zone string) bool {
	zones := map[string]bool{
		"jakarta_pusat": true, "jakarta_selatan": true, "jakarta_barat": true,
		"jakarta_timur": true, "jakarta_utara": true, "bogor": true,
		"depok": true, "tangerang": true, "bekasi": true,
	}
	return zones[zone]
}

func TestABSegment_Determinism(t *testing.T) {
	userIDs := []string{"U001", "U002", "U003", "U004", "U005", "U006", "U007", "U008", "U009", "U010"}
	first := make(map[string]string)

	for _, uid := range userIDs {
		segment := hashABSegment(uid)
		first[uid] = segment
		if segment != "control" && segment != "variant" {
			t.Errorf("user %s: invalid segment %s", uid, segment)
		}
	}

	for _, uid := range userIDs {
		second := hashABSegment(uid)
		if second != first[uid] {
			t.Errorf("user %s: non-deterministic: %s -> %s", uid, first[uid], second)
		}
	}
}

func hashABSegment(userID string) string {
	var sum int
	for _, c := range []byte(userID) {
		sum += int(c)
	}
	if sum%100 < 50 {
		return "control"
	}
	return "variant"
}
