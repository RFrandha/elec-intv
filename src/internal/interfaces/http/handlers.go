package http

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/RFrandha/elec-intv/src/internal/application"
	"github.com/RFrandha/elec-intv/src/internal/domain"
	"github.com/RFrandha/elec-intv/src/internal/infrastructure/database"
	fleet "github.com/RFrandha/elec-intv/src/internal/infrastructure/fleet"
	"github.com/RFrandha/elec-intv/src/internal/interfaces/dto"
)

type Handler struct {
	pricingService *service.PricingService
	configRepo     *database.ConfigRepo
	eventRepo      *database.EventRepo
	fleetRepo      *database.FleetStateRepo
	auditRepo      *database.AuditRepo
	fleetSimulator *fleet.Simulator
}

func NewHandler(
	pricingService *service.PricingService,
	configRepo *database.ConfigRepo,
	eventRepo *database.EventRepo,
	fleetRepo *database.FleetStateRepo,
	auditRepo *database.AuditRepo,
	fleetSimulator *fleet.Simulator,
) *Handler {
	return &Handler{
		pricingService: pricingService,
		configRepo:     configRepo,
		eventRepo:      eventRepo,
		fleetRepo:      fleetRepo,
		auditRepo:      auditRepo,
		fleetSimulator: fleetSimulator,
	}
}

func (h *Handler) GetPricing(c *gin.Context) {
	vehicleID := c.Query("vehicle_id")
	zone := c.Query("zone")
	durationStr := c.Query("duration_hours")

	if vehicleID == "" || zone == "" || durationStr == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "missing required parameters"})
		return
	}

	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil || duration <= 0 || duration > 5.0 {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid duration_hours: must be 0.1-5.0"})
		return
	}

	validZones := map[string]bool{
		"jakarta_pusat": true, "jakarta_selatan": true, "jakarta_barat": true,
		"jakarta_timur": true, "jakarta_utara": true, "bogor": true,
		"depok": true, "tangerang": true, "bekasi": true,
	}
	if !validZones[zone] {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid zone"})
		return
	}

	req := domain.PricingRequest{
		VehicleID:     vehicleID,
		Zone:          zone,
		DurationHours: duration,
	}

	result, err := h.pricingService.Calculate(req)
	if err != nil {
		log.Printf("Pricing failed: %v", err)
		if err.Error() == "vehicle not found: "+vehicleID {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "pricing calculation failed"})
		}
		return
	}

	c.JSON(http.StatusOK, dto.NewPricingResponse(result))
}

func (h *Handler) GetPricingBreakdown(c *gin.Context) {
	quoteID := c.Param("quote_id")
	if quoteID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "missing quote_id"})
		return
	}

	c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "breakdown endpoint pending implementation"})
}

func (h *Handler) GetAdminConfig(c *gin.Context) {
	config, err := h.configRepo.FindActive()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to load config"})
		return
	}

	c.JSON(http.StatusOK, dto.ConfigResponse{
		BasePrice:            config.BasePrice,
		DemandRules:          config.DemandRules,
		ZoneSurgeThresholds:  config.ZoneSurgeThresholds,
		BatteryDiscountTiers: config.BatteryDiscountTiers,
	})
}

func (h *Handler) UpdateAdminConfig(c *gin.Context) {
	var input domain.ConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	if input.BasePrice <= 0 || input.BasePrice > 100000 {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "base_price must be positive and < 100000"})
		return
	}

	if err := h.configRepo.Create(input); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to save config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config saved, will activate within 30 seconds"})
}

func (h *Handler) GetConfigHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	configs, err := h.configRepo.FindHistory(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to load history"})
		return
	}

	var response []dto.ConfigHistoryResponse
	for _, cfg := range configs {
		response = append(response, dto.ConfigHistoryResponse{
			ConfigID:  cfg.ConfigID,
			Version:   cfg.Version,
			CreatedAt: cfg.CreatedAt.Format(time.RFC3339),
			CreatedBy: cfg.CreatedBy,
		})
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) GetAdminFleetState(c *gin.Context) {
	states, err := h.fleetRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to load fleet state"})
		return
	}

	var response []dto.FleetStateResponse
	for _, s := range states {
		response = append(response, dto.FleetStateResponse{
			Zone:              s.Zone,
			TotalVehicles:     s.TotalVehicles,
			AvailableVehicles: s.AvailableVehicles,
			UtilizationPct:    s.Utilization() * 100,
			BSSCount:          s.BSSCount,
		})
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) ListEvents(c *gin.Context) {
	events, err := h.eventRepo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list events"})
		return
	}

	now := time.Now()
	var response []dto.EventResponse
	for _, e := range events {
		response = append(response, dto.EventResponse{
			ID:         e.ID,
			Name:       e.Name,
			Zone:       e.Zone,
			BSSID:      e.BSSID,
			StartTime:  e.StartTime.Format(time.RFC3339),
			EndTime:    e.EndTime.Format(time.RFC3339),
			Multiplier: e.DiscountMultiplier,
			IsActive:   e.IsActive(now),
		})
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) CreateEvent(c *gin.Context) {
	var input domain.EventInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})
		return
	}

	if input.DiscountMultiplier <= 0 || input.DiscountMultiplier > 2.0 {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "discount_multiplier must be 0.01-2.0"})
		return
	}

	eventID, err := h.eventRepo.Create(input)
	if err != nil {
		log.Printf("Failed to create event: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to create event"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"event_id": eventID, "message": "event created"})
}

func (h *Handler) DeleteEvent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid event_id"})
		return
	}

	if err := h.eventRepo.Delete(id); err != nil {
		if err.Error() == "event not found" {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "event not found"})
		} else {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to delete event"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "event deleted"})
}

func (h *Handler) GetABStats(c *gin.Context) {
	stats, err := h.auditRepo.FindABStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to load stats"})
		return
	}

	c.JSON(http.StatusOK, dto.ABStatsResponse{
		Control: dto.SegmentStats{
			Requests:     stats.Control.Requests,
			AvgPrice:     stats.Control.AvgPrice,
			TotalRevenue: stats.Control.TotalRevenue,
		},
		Variant: dto.SegmentStats{
			Requests:     stats.Variant.Requests,
			AvgPrice:     stats.Variant.AvgPrice,
			TotalRevenue: stats.Variant.TotalRevenue,
		},
	})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "missing X-API-Key header"})
			c.Abort()
			return
		}

		readOnlyKey := os.Getenv("READ_ONLY_API_KEY")
		if readOnlyKey == "" {
			readOnlyKey = "demo-read-only-1234"
		}
		adminKey := os.Getenv("ADMIN_API_KEY")
		if adminKey == "" {
			adminKey = "admin-secure-key-5678"
		}

		if key != readOnlyKey && key != adminKey {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		adminKey := os.Getenv("ADMIN_API_KEY")
		if adminKey == "" {
			adminKey = "admin-secure-key-5678"
		}

		if key != adminKey {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "unauthorized, admin key required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (h *Handler) RefreshConfig(c *gin.Context) {
	h.pricingService.RefreshNow()
	c.JSON(http.StatusOK, gin.H{"message": "config/events/tiers refreshed"})
}

func (h *Handler) RefreshFleet(c *gin.Context) {
	h.fleetSimulator.RefreshOnce()
	c.JSON(http.StatusOK, gin.H{"message": "fleet state refreshed"})
}
