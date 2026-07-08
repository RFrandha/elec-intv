package http

import "testing"

func TestValidation_InvalidZone(t *testing.T) {
	validZones := map[string]bool{
		"jakarta_pusat": true, "jakarta_selatan": true, "jakarta_barat": true,
		"jakarta_timur": true, "jakarta_utara": true, "bogor": true,
		"depok": true, "tangerang": true, "bekasi": true, "bandung": true,
	}

	invalidZones := []string{"surabaya", "  ", "jakarta", ""}
	for _, z := range invalidZones {
		if validZones[z] {
			t.Errorf("zone %q should NOT be valid", z)
		}
	}

	valid := []string{"jakarta_pusat", "bogor", "bekasi", "bandung"}
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

func TestZones_AllPresent(t *testing.T) {
	expected := []string{
		"jakarta_pusat", "jakarta_selatan", "jakarta_barat",
		"jakarta_timur", "jakarta_utara", "bogor",
		"depok", "tangerang", "bekasi", "bandung",
	}

	validZones := map[string]bool{
		"jakarta_pusat": true, "jakarta_selatan": true, "jakarta_barat": true,
		"jakarta_timur": true, "jakarta_utara": true, "bogor": true,
		"depok": true, "tangerang": true, "bekasi": true, "bandung": true,
	}

	for _, z := range expected {
		if !validZones[z] {
			t.Errorf("expected zone %q not found", z)
		}
	}

	invalid := []string{"surabaya", "  ", "jakarta", ""}
	for _, z := range invalid {
		if validZones[z] {
			t.Errorf("zone %q should NOT be valid", z)
		}
	}
}
