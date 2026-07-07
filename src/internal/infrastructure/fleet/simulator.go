package fleet

import (
	"log"
	"math"
	"time"

	"github.com/RFrandha/elec-intv/src/internal/infrastructure/database"
)

type Simulator struct {
	repo   *database.FleetStateRepo
	stopCh chan struct{}
}

func NewSimulator(repo *database.FleetStateRepo) *Simulator {
	return &Simulator{
		repo:   repo,
		stopCh: make(chan struct{}),
	}
}

func (s *Simulator) Start() {
	go func() {
		log.Println("Fleet simulator started")
		ticker := time.NewTicker(30 * time.Second)

		for {
			select {
			case <-ticker.C:
				s.updateFleetState()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *Simulator) Stop() {
	close(s.stopCh)
}

func (s *Simulator) updateFleetState() {
	hour := time.Now().Hour()
	weekday := time.Now().Weekday() != time.Saturday && time.Now().Weekday() != time.Sunday

	var peakFactor float64
	switch {
	case weekday && hour >= 7 && hour <= 9:
		peakFactor = 0.9
	case weekday && hour >= 17 && hour <= 19:
		peakFactor = 0.9
	case weekday && hour >= 22 || hour <= 5:
		peakFactor = 0.3
	case !weekday && hour >= 10 && hour <= 18:
		peakFactor = 0.7
	default:
		peakFactor = 0.5
	}

	zones := []string{"jakarta_pusat", "jakarta_selatan", "jakarta_barat", "jakarta_timur", "jakarta_utara", "bogor", "depok", "tangerang", "bekasi"}

	zoneBaseUtil := map[string]float64{
		"jakarta_pusat": 0.85, "jakarta_selatan": 0.75, "jakarta_barat": 0.70,
		"jakarta_timur": 0.65, "jakarta_utara": 0.60, "bogor": 0.50,
		"depok": 0.55, "tangerang": 0.70, "bekasi": 0.65,
	}

	for _, zone := range zones {
		baseUtil := zoneBaseUtil[zone]
		variation := math.Sin(float64(time.Now().Unix()/30)+float64(len(zone))*2.5) * 0.1
		utilization := math.Min(1.0, math.Max(0.0, baseUtil*peakFactor+variation))

		totalVehicles := 20
		availableVehicles := int(math.Round(float64(totalVehicles) * (1.0 - utilization)))
		if availableVehicles < 0 {
			availableVehicles = 0
		}
		if availableVehicles > totalVehicles {
			availableVehicles = totalVehicles
		}

		if err := s.repo.UpdateUtilization(zone, availableVehicles, totalVehicles); err != nil {
			log.Printf("Fleet sim: %s update failed: %v", zone, err)
		}
	}
}
