package main

import (
	"log"
	"os"

	service "github.com/RFrandha/elec-intv/src/internal/application"
	"github.com/RFrandha/elec-intv/src/internal/infrastructure/cache"
	"github.com/RFrandha/elec-intv/src/internal/infrastructure/database"
	fleet "github.com/RFrandha/elec-intv/src/internal/infrastructure/fleet"
	httpHandler "github.com/RFrandha/elec-intv/src/internal/interfaces/http"
)

func main() {
	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		hmacSecret = "audit-hmac-secret-9012"
		log.Println("WARNING: using default HMAC_SECRET")
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.DB.Close()

	cacheStore := cache.NewInMemory()

	vehicleRepo := database.NewVehicleRepo(db)
	userRepo := database.NewUserRepo(db)
	fleetRepo := database.NewFleetStateRepo(db)
	eventRepo := database.NewEventRepo(db)
	auditRepo := database.NewAuditRepo(db)
	configRepo := database.NewConfigRepo(db)
	tierRepo := database.NewTierRepo(db)

	pricingService := service.NewPricingService(
		cacheStore, vehicleRepo, userRepo, fleetRepo,
		eventRepo, auditRepo, configRepo, tierRepo, hmacSecret,
	)

	pricingService.StartCacheUpdater()

	simulator := fleet.NewSimulator(fleetRepo)
	simulator.Start()

	handler := httpHandler.NewHandler(
		pricingService, configRepo, eventRepo, fleetRepo, auditRepo,
	)

	router := httpHandler.SetupRouter(handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting pricing engine on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
