package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/server"
	_ "github.com/joho/godotenv/autoload"
)

// main is the entry point for the application.
// It sets up the MCP server with the appropriate handlers and starts it.
func main() {
	// Define command-line flags for configuration override
	deepseekModelFlag := flag.String("deepseek-model", "", "DeepSeek model name (overrides env var)")
	deepseekSystemPromptFlag := flag.String("deepseek-system-prompt", "", "System prompt (overrides env var)")
	deepseekTemperatureFlag := flag.Float64("deepseek-temperature", -1, "Temperature setting (0.0-1.0, overrides env var)")
	flag.Parse()

	// Create application context with logger
	logger := NewLogger(LevelInfo)
	ctx := context.WithValue(context.Background(), loggerKey, logger)

	// Create configuration from environment variables
	config, err := NewConfig()
	if err != nil {
		handleStartupError(ctx, err)
		return
	}

	// Override with command-line flags if provided
	if *deepseekModelFlag != "" {
		// Validate the model ID before setting it
		if err := ValidateModelID(*deepseekModelFlag); err != nil {
			logger.Error("Invalid model specified: %v", err)
			handleStartupError(ctx, fmt.Errorf("invalid model specified: %w", err))
			return
		}
		logger.Info("Overriding DeepSeek model with flag value: %s", *deepseekModelFlag)
		config.DeepseekModel = *deepseekModelFlag
	}
	if *deepseekSystemPromptFlag != "" {
		logger.Info("Overriding DeepSeek system prompt with flag value")
		config.DeepseekSystemPrompt = *deepseekSystemPromptFlag
	}

	// Override temperature if provided and valid
	if *deepseekTemperatureFlag >= 0 {
		// Validate temperature is within range
		if *deepseekTemperatureFlag > 1.0 {
			logger.Error("Invalid temperature value: %v. Must be between 0.0 and 1.0", *deepseekTemperatureFlag)
			handleStartupError(ctx, fmt.Errorf("invalid temperature: %v", *deepseekTemperatureFlag))
			return
		}
		logger.Info("Overriding DeepSeek temperature with flag value: %v", *deepseekTemperatureFlag)
		config.DeepseekTemperature = float32(*deepseekTemperatureFlag)
	}

	// Store config in context for error handler to access
	ctx = context.WithValue(ctx, configKey, config)

	// Set up handler registry
	// NewHandlerRegistry is a constructor that doesn't return an error
	registry := handler.NewHandlerRegistry()

	// Create and register the DeepSeek server
	if err := setupDeepseekServer(ctx, registry, config); err != nil {
		handleStartupError(ctx, err)
		return
	}

	// Start the MCP server
	srv := server.New(server.Options{
		Name:     "deepseek",
		Version:  "1.0.0",
		Registry: registry,
	})

	logger.Info("Starting DeepSeek MCP server")
	if err := srv.Run(); err != nil {
		logger.Error("Server error: %v", err)
		os.Exit(1)
	}
}



// setupDeepseekServer creates and registers a DeepSeek server
func setupDeepseekServer(ctx context.Context, registry *handler.HandlerRegistry, config *Config) error {
	loggerValue := ctx.Value(loggerKey)
	logger, ok := loggerValue.(Logger)
	if !ok {
		return fmt.Errorf("logger not found in context")
	}

	// Create the DeepSeek server with configuration
	deepseekServer, err := NewDeepseekServer(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create DeepSeek server: %w", err)
	}

	// Wrap the server with logger middleware
	handlerWithLogger := NewLoggerMiddleware(deepseekServer, logger)

	// Register the wrapped server
	registry.RegisterToolHandler(handlerWithLogger)
	logger.Info("Registered DeepSeek server in normal mode with model: %s", config.DeepseekModel)

	// Log file handling configuration
	logger.Info("File handling: max size %s, allowed types: %v",
		humanReadableSize(config.MaxFileSize),
		config.AllowedFileTypes)

	// Log a truncated version of the system prompt for security/brevity
	promptPreview := config.DeepseekSystemPrompt
	if len(promptPreview) > 50 {
		// Use proper UTF-8 safe truncation
		runeCount := 0
		for i := range promptPreview {
			runeCount++
			if runeCount > 50 {
				promptPreview = promptPreview[:i] + "..."
				break
			}
		}
	}
	logger.Info("Using system prompt: %s", promptPreview)

	return nil
}

// handleStartupError handles initialization errors by setting up an error server
func handleStartupError(ctx context.Context, err error) {
	// Safely extract logger from context
	loggerValue := ctx.Value(loggerKey)
	logger, ok := loggerValue.(Logger)
	if !ok {
		// Fallback to a new logger if type assertion fails
		logger = NewLogger(LevelError)
	}
	errorMsg := err.Error()

	logger.Error("Initialization error: %v", err)

	// Get config for EnableCaching status (if available)
	var config *Config
	configValue := ctx.Value(configKey)
	if configValue != nil {
		if cfg, ok := configValue.(*Config); ok {
			config = cfg
		}
	}

	// Create error server
	errorServer := &ErrorDeepseekServer{
		errorMessage: errorMsg,
		config:       config,
	}

	// Set up registry with error server
	// NewHandlerRegistry is a constructor that doesn't return an error
	registry := handler.NewHandlerRegistry()
	errorServerWithLogger := NewLoggerMiddleware(errorServer, logger)
	registry.RegisterToolHandler(errorServerWithLogger)

	// Start server in degraded mode
	logger.Info("Starting DeepSeek MCP server in degraded mode")
	srv := server.New(server.Options{
		Name:     "deepseek",
		Version:  "1.0.0",
		Registry: registry,
	})

	if err := srv.Run(); err != nil {
		logger.Error("Server error in degraded mode: %v", err)
		os.Exit(1)
	}
}
