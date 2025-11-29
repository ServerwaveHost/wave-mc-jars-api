package providers

import (
	"fmt"
	"sync"
)

// Registry manages all available providers
type Registry struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewRegistry creates a new provider registry with all default providers
func NewRegistry(config ProviderConfig) *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
	}

	// Register game server providers
	r.Register(NewVanillaProvider(config))
	r.Register(NewPaperProvider(config))
	r.Register(NewFoliaProvider(config))
	r.Register(NewPurpurProvider(config))

	// Register proxy providers
	r.Register(NewVelocityProvider(config))
	r.Register(NewWaterfallProvider(config))
	r.Register(NewBungeeCordProvider(config))

	return r
}

// Register adds a provider to the registry
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.GetID()] = p
}

// Get returns a provider by ID
func (r *Registry) Get(id string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", id)
	}
	return p, nil
}

// List returns all registered providers
func (r *Registry) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// ListIDs returns all registered provider IDs
func (r *Registry) ListIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}
