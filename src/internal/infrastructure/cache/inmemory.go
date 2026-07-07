package cache

import (
	"sync"
	"time"

	"github.com/RFrandha/elec-intv/src/internal/domain"
)

type InMemory struct {
	mu           sync.RWMutex
	Config       *domain.PricingConfig
	Events       []domain.Event
	Tiers        map[string]*domain.Tier
	ConfigLoaded time.Time
	EventsLoaded time.Time
	TiersLoaded  time.Time
}

func NewInMemory() *InMemory {
	return &InMemory{
		Config: &domain.PricingConfig{BasePrice: 4000},
		Tiers: map[string]*domain.Tier{
			"normal": {ID: "normal", DiscountMultiplier: 1.0},
			"gold":   {ID: "gold", DiscountMultiplier: 0.9},
		},
	}
}

func (c *InMemory) GetConfig() *domain.PricingConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Config
}

func (c *InMemory) SetConfig(config *domain.PricingConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Config = config
	c.ConfigLoaded = time.Now()
}

func (c *InMemory) GetEvents() []domain.Event {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Events
}

func (c *InMemory) SetEvents(events []domain.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Events = events
	c.EventsLoaded = time.Now()
}

func (c *InMemory) GetTiers() map[string]*domain.Tier {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Tiers
}

func (c *InMemory) SetTiers(tiers []domain.Tier) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := make(map[string]*domain.Tier)
	for i := range tiers {
		m[tiers[i].ID] = &tiers[i]
	}
	c.Tiers = m
	c.TiersLoaded = time.Now()
}
