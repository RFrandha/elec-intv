package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/RFrandha/elec-intv/src/internal/domain"
)

type DB struct {
	*sql.DB
}

func Connect() (*DB, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}

	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping: %w", err)
	}

	log.Println("Connected to database")

	db := &DB{conn}
	if err := db.runMigrations(); err != nil {
		return nil, err
	}

	if err := db.seedData(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) runMigrations() error {
	if _, err := db.Exec("CREATE SCHEMA IF NOT EXISTS pricing"); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	paths := []string{"migrations/001_initial_schema.sql", "./migrations/001_initial_schema.sql"}
	var migrationSQL []byte
	var err error

	for _, p := range paths {
		migrationSQL, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}

	if err != nil {
		return fmt.Errorf("failed to read migration: %w", err)
	}

	if _, err := db.Exec(string(migrationSQL)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	log.Println("Migrations applied")
	return nil
}

func (db *DB) seedData() error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM pricing.users").Scan(&count); err != nil {
		return err
	}

	if count > 0 {
		log.Println("Seed data already exists")
		return nil
	}

	users := []struct {
		id   string
		tier string
	}{
		{"U001", "gold"},
		{"U002", "normal"},
		{"U003", "gold"},
	}

	for _, u := range users {
		_, err := db.Exec("INSERT INTO pricing.users (user_id, subscription_tier_id) VALUES ($1, $2)", u.id, u.tier)
		if err != nil {
			return err
		}
	}

	zones := []string{"jakarta_pusat", "jakarta_selatan", "jakarta_barat", "jakarta_timur", "jakarta_utara", "bogor", "depok", "tangerang", "bekasi"}

	for i, zone := range zones {
		total := 100 + i*10
		available := total - (i * 8)
		bss := 30 + i*5
		_, err := db.Exec(
			`INSERT INTO pricing.fleet_state (zone, total_vehicles, available_vehicles, bss_count) VALUES ($1, $2, $3, $4)`,
			zone, total, available, bss,
		)
		if err != nil {
			return err
		}
	}

	vehicles := []struct {
		id     string
		zone   string
		soc    float64
		userID string
	}{
		{"V001", "jakarta_pusat", 30.0, "U001"},
		{"V002", "jakarta_selatan", 75.0, "U002"},
		{"V003", "jakarta_barat", 50.0, "U003"},
		{"V004", "jakarta_timur", 90.0, "U001"},
		{"V005", "jakarta_utara", 35.0, "U002"},
		{"V006", "bogor", 60.0, "U003"},
		{"V007", "depok", 45.0, "U001"},
		{"V008", "tangerang", 80.0, "U002"},
	}

	for _, v := range vehicles {
		_, err := db.Exec(
			`INSERT INTO pricing.vehicles (vehicle_id, zone, current_soc, current_user_id, model, last_swap_timestamp)
			 VALUES ($1, $2, $3, $4, 'H1', NOW() - INTERVAL '2 hours')`,
			v.id, v.zone, v.soc, v.userID,
		)
		if err != nil {
			return err
		}
	}

	log.Println("Seed data inserted")
	return nil
}

type VehicleRepo struct {
	db *DB
}

func NewVehicleRepo(db *DB) *VehicleRepo {
	return &VehicleRepo{db: db}
}

func (r *VehicleRepo) FindByID(id string) (*domain.Vehicle, error) {
	var v domain.Vehicle
	err := r.db.QueryRow(
		`SELECT vehicle_id, zone, current_soc, current_user_id, model, last_swap_timestamp, last_updated
		 FROM pricing.vehicles WHERE vehicle_id = $1`, id,
	).Scan(&v.ID, &v.Zone, &v.CurrentSOC, &v.CurrentUserID, &v.Model, &v.LastSwap, &v.LastUpdated)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("vehicle not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) FindByID(id string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(
		`SELECT user_id, subscription_tier_id, rental_count, created_at
		 FROM pricing.users WHERE user_id = $1`, id,
	).Scan(&u.ID, &u.SubscriptionTier, &u.RentalCount, &u.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

type FleetStateRepo struct {
	db *DB
}

func NewFleetStateRepo(db *DB) *FleetStateRepo {
	return &FleetStateRepo{db: db}
}

func (r *FleetStateRepo) FindByZone(zone string) (*domain.FleetState, error) {
	var f domain.FleetState
	err := r.db.QueryRow(
		`SELECT zone, total_vehicles, available_vehicles, bss_count, updated_at
		 FROM pricing.fleet_state WHERE zone = $1`, zone,
	).Scan(&f.Zone, &f.TotalVehicles, &f.AvailableVehicles, &f.BSSCount, &f.UpdatedAt)

	if err == sql.ErrNoRows {
		return &domain.FleetState{Zone: zone, TotalVehicles: 100, AvailableVehicles: 50, BSSCount: 30}, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FleetStateRepo) FindAll() ([]domain.FleetState, error) {
	rows, err := r.db.Query(`SELECT zone, total_vehicles, available_vehicles, bss_count, updated_at FROM pricing.fleet_state ORDER BY zone`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []domain.FleetState
	for rows.Next() {
		var f domain.FleetState
		if err := rows.Scan(&f.Zone, &f.TotalVehicles, &f.AvailableVehicles, &f.BSSCount, &f.UpdatedAt); err != nil {
			continue
		}
		states = append(states, f)
	}
	return states, nil
}

func (r *FleetStateRepo) UpdateUtilization(zone string, available, total int) error {
	_, err := r.db.Exec(
		`INSERT INTO pricing.fleet_state (zone, total_vehicles, available_vehicles, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (zone) DO UPDATE SET available_vehicles = $3, updated_at = NOW()`,
		zone, total, available,
	)
	return err
}
