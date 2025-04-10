package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// CacheRequest represents a request to create a cached context
type CacheRequest struct {
	Model        string   `json:"model"`        // Model ID to use
	SystemPrompt string   `json:"system_prompt"` // System prompt
	FilePaths    []string `json:"file_paths"`    // File paths to include in the cache
	TTL          string   `json:"ttl"`           // Cache TTL (e.g., "1h", "24h", etc.)
}

// createCache creates a new cache entry
func (s *DeepseekServer) createCache(ctx context.Context, req *CacheRequest) (*CacheInfo, error) {
	logger := getLoggerFromContext(ctx)
	
	// Check if caching is enabled
	if !s.config.EnableCaching {
		return nil, errors.New("caching is disabled")
	}
	
	// Input validation
	if req.Model == "" {
		return nil, errors.New("model is required")
	}
	
	// Validate the model
	if err := ValidateModelID(req.Model); err != nil {
		return nil, fmt.Errorf("invalid model: %w", err)
	}
	
	// Parse TTL
	var ttl time.Duration
	if req.TTL == "" {
		ttl = s.config.DefaultCacheTTL
	} else {
		var err error
		ttl, err = time.ParseDuration(req.TTL)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL format: %w", err)
		}
	}
	
	// Generate a unique cache ID
	cacheID := fmt.Sprintf("cache-%d", time.Now().UnixNano())
	
	// Create expiration time
	createdAt := time.Now()
	expiresAt := createdAt.Add(ttl)
	
	// Create cache info
	cacheInfo := &CacheInfo{
		ID:          cacheID,
		SystemPrompt: req.SystemPrompt,
		Model:       req.Model,
		FilePaths:   req.FilePaths,
		CreatedAt:   createdAt,
		ExpiresAt:   expiresAt,
	}
	
	// Store cache info
	s.cacheMu.Lock()
	s.caches[cacheID] = cacheInfo
	s.cacheMu.Unlock()
	
	logger.Info("Cache created successfully with ID: %s, expires at: %v", cacheID, expiresAt)
	return cacheInfo, nil
}

// getCache gets cache information by ID
func (s *DeepseekServer) getCache(ctx context.Context, id string) (*CacheInfo, error) {
	logger := getLoggerFromContext(ctx)

	// Check cache 
	s.cacheMu.RLock()
	info, ok := s.caches[id]
	s.cacheMu.RUnlock()

	if ok {
		// Check if cache has expired
		if time.Now().After(info.ExpiresAt) {
			s.cacheMu.Lock()
			delete(s.caches, id)
			s.cacheMu.Unlock()
			logger.Info("Cache %s has expired", id)
			return nil, fmt.Errorf("cache has expired")
		}
		
		logger.Debug("Cache info for %s found", id)
		return info, nil
	}

	// If not in cache, return not found error
	return nil, fmt.Errorf("cache not found: %s", id)
}
