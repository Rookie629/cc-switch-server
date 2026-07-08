package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/Rookie629/cc-switch/internal/store"
)

// ProviderService handles all business logic for provider management.
type ProviderService struct {
	store *store.Store
}

// NewProviderService creates a new ProviderService.
func NewProviderService(s *store.Store) *ProviderService {
	return &ProviderService{store: s}
}

// List returns all providers.
func (ps *ProviderService) List() ([]store.Provider, error) {
	pf, err := ps.store.Read()
	if err != nil {
		return nil, err
	}
	return pf.Providers, nil
}

// GetActive returns the currently active provider.
func (ps *ProviderService) GetActive() (*store.Provider, error) {
	pf, err := ps.store.Read()
	if err != nil {
		return nil, err
	}
	return store.ActiveProvider(pf), nil
}

// GetByName returns a provider by name.
func (ps *ProviderService) GetByName(name string) (*store.Provider, error) {
	pf, err := ps.store.Read()
	if err != nil {
		return nil, err
	}
	p, _ := store.FindByName(pf, name)
	if p == nil {
		return nil, store.ErrProviderNotFound
	}
	return p, nil
}

// Add creates a new provider. Returns error if name already exists.
func (ps *ProviderService) Add(name, providerType, apiKey, baseURL string, models []string, defaultModel string) (*store.Provider, error) {
	if name == "" || apiKey == "" || baseURL == "" {
		return nil, errors.New("name, api_key, and base_url are required")
	}

	t := store.ProviderType(providerType)
	switch t {
	case store.TypeAnthropic, store.TypeOpenAICompatible, store.TypeOpenAI:
		// valid
	default:
		return nil, fmt.Errorf("invalid provider type: %s (must be anthropic, openai_compatible, or openai)", providerType)
	}

	pf, err := ps.store.Read()
	if err != nil {
		return nil, err
	}

	if p, _ := store.FindByName(pf, name); p != nil {
		return nil, store.ErrProviderAlreadyExists
	}

	if models == nil {
		models = []string{}
	}
	if defaultModel == "" && len(models) > 0 {
		defaultModel = models[0]
	}

	now := time.Now()
	provider := store.Provider{
		ID:           uuid.New().String()[:8],
		Name:         name,
		Type:         t,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Models:       models,
		DefaultModel: defaultModel,
		IsActive:     false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// First provider? Make it active.
	if len(pf.Providers) == 0 {
		provider.IsActive = true
		pf.ActiveProviderID = provider.ID
	}

	pf.Providers = append(pf.Providers, provider)

	if err := ps.store.Write(pf); err != nil {
		return nil, err
	}
	return &provider, nil
}

// Remove deletes a provider by name. Refuses to delete the active provider.
func (ps *ProviderService) Remove(name string) error {
	pf, err := ps.store.Read()
	if err != nil {
		return err
	}

	p, idx := store.FindByName(pf, name)
	if p == nil {
		return store.ErrProviderNotFound
	}
	if p.IsActive {
		return store.ErrProviderActive
	}

	pf.Providers = append(pf.Providers[:idx], pf.Providers[idx+1:]...)
	return ps.store.Write(pf)
}

// SetActive switches the active provider by name.
func (ps *ProviderService) SetActive(name string) (*store.Provider, error) {
	pf, err := ps.store.Read()
	if err != nil {
		return nil, err
	}

	target, _ := store.FindByName(pf, name)
	if target == nil {
		return nil, store.ErrProviderNotFound
	}

	// Deactivate all, then activate the target
	for i := range pf.Providers {
		pf.Providers[i].IsActive = false
	}

	target.IsActive = true
	target.UpdatedAt = time.Now()
	pf.ActiveProviderID = target.ID

	if err := ps.store.Write(pf); err != nil {
		return nil, err
	}
	return target, nil
}

// Edit updates an existing provider's fields.
func (ps *ProviderService) Edit(name string, updates map[string]interface{}) (*store.Provider, error) {
	pf, err := ps.store.Read()
	if err != nil {
		return nil, err
	}

	p, _ := store.FindByName(pf, name)
	if p == nil {
		return nil, store.ErrProviderNotFound
	}

	if v, ok := updates["name"].(string); ok && v != "" {
		p.Name = v
	}
	if v, ok := updates["type"].(string); ok && v != "" {
		p.Type = store.ProviderType(v)
	}
	if v, ok := updates["api_key"].(string); ok && v != "" {
		p.APIKey = v
	}
	if v, ok := updates["base_url"].(string); ok && v != "" {
		p.BaseURL = v
	}
	if v, ok := updates["models"].([]string); ok {
		p.Models = v
	}
	if v, ok := updates["default_model"].(string); ok && v != "" {
		p.DefaultModel = v
	}

	p.UpdatedAt = time.Now()

	if err := ps.store.Write(pf); err != nil {
		return nil, err
	}
	return p, nil
}

// GetStore returns the underlying store (used by config_writer and proxy).
func (ps *ProviderService) GetStore() *store.Store {
	return ps.store
}
