package http

import (
	"github.com/gin-gonic/gin"
)

func SetupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	api := r.Group("/api/v1")

	// Public endpoints (require read-only or admin key)
	pricing := api.Group("/pricing")
	pricing.Use(AuthMiddleware())
	{
		pricing.GET("", h.GetPricing)
		pricing.GET("/:quote_id/breakdown", h.GetPricingBreakdown)
	}

	// Admin endpoints (require admin key)
	admin := api.Group("/admin")
	admin.Use(AdminAuthMiddleware())
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
