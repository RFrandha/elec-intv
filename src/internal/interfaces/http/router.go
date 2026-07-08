package http

import (
	"time"

	"github.com/gin-gonic/gin"
)

func SetupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")

	// Public endpoints (require read-only or admin key)
	pricing := api.Group("/pricing")
	pricing.Use(AuthMiddleware())
	pricing.Use(RateLimitMiddleware(100, time.Minute))
	{
		pricing.GET("", h.GetPricing)
		pricing.GET("/:quote_id/breakdown", h.GetPricingBreakdown)
	}

	// Admin endpoints (require admin key)
	admin := api.Group("/admin")
	admin.Use(AdminAuthMiddleware())
	admin.Use(RateLimitMiddleware(10, time.Minute))
	{
		admin.GET("/config", h.GetAdminConfig)
		admin.PUT("/config", h.UpdateAdminConfig)
		admin.GET("/config/history", h.GetConfigHistory)
		admin.POST("/config/refresh", h.RefreshConfig)
		admin.GET("/fleet/state", h.GetAdminFleetState)
		admin.POST("/fleet/refresh", h.RefreshFleet)
		admin.GET("/events", h.ListEvents)
		admin.POST("/events", h.CreateEvent)
		admin.DELETE("/events/:id", h.DeleteEvent)
		admin.GET("/stats/ab-tests", h.GetABStats)
		admin.GET("/stats/pricing", h.GetPricingStats)
		admin.GET("/ab-tests", h.ListABTests)
		admin.DELETE("/ab-tests/:id", h.DeleteABTest)
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	}
}
