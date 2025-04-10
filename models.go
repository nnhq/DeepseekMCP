package main

import (
	"errors"
	"fmt"
	"strings"
)

// DeepseekModelInfo holds information about a DeepSeek model
type DeepseekModelInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	SupportsCaching bool   `json:"supports_caching"` // Whether this model supports caching
}

// GetModelByID returns a specific model by ID, or nil if not found
func GetModelByID(modelID string) *DeepseekModelInfo {
	models := GetAvailableDeepseekModels()
	for _, model := range models {
		if model.ID == modelID {
			return &model
		}
	}
	return nil
}

// ValidateModelID checks if a model ID is in the list of available models
// Returns nil if valid, error otherwise
func ValidateModelID(modelID string) error {
	if GetModelByID(modelID) != nil {
		return nil
	}

	// Model not found, return error with available models
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Invalid model ID: %s. Available models are:", modelID))
	for _, model := range GetAvailableDeepseekModels() {
		sb.WriteString(fmt.Sprintf("\n- %s: %s", model.ID, model.Name))
	}

	return errors.New(sb.String())
}

// GetAvailableDeepseekModels returns a list of available DeepSeek models
func GetAvailableDeepseekModels() []DeepseekModelInfo {
	return []DeepseekModelInfo{
		{
			ID:              "deepseek-chat",
			Name:            "DeepSeek Chat",
			Description:     "General-purpose chat model from DeepSeek, balancing performance and efficiency",
			SupportsCaching: false,
		},
		{
			ID:              "deepseek-coder",
			Name:            "DeepSeek Coder",
			Description:     "Specialized model for coding and technical tasks",
			SupportsCaching: false,
		},
		{
			ID:              "deepseek-reasoner",
			Name:            "DeepSeek Reasoner",
			Description:     "Model optimized for reasoning and problem-solving tasks",
			SupportsCaching: false,
		},
		{
			ID:              "deepseek-chat-001",
			Name:            "DeepSeek Chat (Stable)",
			Description:     "Stable version of DeepSeek Chat with version suffix",
			SupportsCaching: true, // Has version suffix
		},
		{
			ID:              "deepseek-coder-001",
			Name:            "DeepSeek Coder (Stable)",
			Description:     "Stable version of DeepSeek Coder with version suffix",
			SupportsCaching: true, // Has version suffix
		},
	}
}
