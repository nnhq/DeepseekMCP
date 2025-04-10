package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cohesion-org/deepseek-go"
	"github.com/gomcpgo/mcp/pkg/protocol"
)

// DeepseekServer implements the ToolHandler interface for DeepSeek API interactions
type DeepseekServer struct {
	config  *Config
	client  *deepseek.Client
	caches  map[string]*CacheInfo // In-memory cache storage
	cacheMu sync.RWMutex          // Mutex for thread-safe cache access
}

// CacheInfo represents information about a cached context
type CacheInfo struct {
	ID          string    `json:"id"`           // Unique ID for the cache
	SystemPrompt string    `json:"system_prompt"` // System prompt used with this cache
	Model       string    `json:"model"`         // Model used with this cache
	FilePaths   []string  `json:"file_paths"`    // File paths included in the cache
	CreatedAt   time.Time `json:"created_at"`    // When the cache was created
	ExpiresAt   time.Time `json:"expires_at"`    // When the cache expires
}

// NewDeepseekServer creates a new DeepseekServer with the provided configuration
func NewDeepseekServer(ctx context.Context, config *Config) (*DeepseekServer, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if config.DeepseekAPIKey == "" {
		return nil, errors.New("DeepSeek API key is required")
	}

	// Initialize the DeepSeek client
	client := deepseek.NewClient(config.DeepseekAPIKey)
	
	// No error is returned by NewClient in the current library version

	// Create a simplified DeepseekServer without dedicated file/cache stores
	return &DeepseekServer{
		config: config,
		client: client,
		caches: make(map[string]*CacheInfo), // Initialize an in-memory cache map
	}, nil
}

// Close closes the DeepSeek client connection (not needed for the DeepSeek API)
func (s *DeepseekServer) Close() {
	// No need to close the client in the DeepSeek API
}

// ListTools implements the ToolHandler interface for DeepseekServer
func (s *DeepseekServer) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	tools := []protocol.Tool{
		{
			Name:        "deepseek_ask",
			Description: "Use DeepSeek's AI model to ask about complex coding problems",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "The coding problem that we are asking DeepSeek AI to work on [question + code]"
					},
					"model": {
						"type": "string",
						"description": "Optional: Specific DeepSeek model to use (overrides default configuration)"
					},
					"systemPrompt": {
						"type": "string",
						"description": "Optional: Custom system prompt to use for this request (overrides default configuration)"
					},
					"file_paths": {
						"type": "array",
						"items": {
							"type": "string"
						},
						"description": "Optional: Paths to files to include in the request context"
					},
					"use_cache": {
						"type": "boolean",
						"description": "Optional: Whether to try using a cache for this request (only works with compatible models)"
					},
					"cache_ttl": {
						"type": "string",
						"description": "Optional: TTL for cache if created (e.g., '10m', '1h'). Default is 10 minutes"
					}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "deepseek_models",
			Description: "List available DeepSeek models with descriptions",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
	}

	return &protocol.ListToolsResponse{
		Tools: tools,
	}, nil
}

// getLoggerFromContext safely extracts a logger from the context or creates a new one
func getLoggerFromContext(ctx context.Context) Logger {
	loggerValue := ctx.Value(loggerKey)
	if loggerValue != nil {
		if l, ok := loggerValue.(Logger); ok {
			return l
		}
	}
	// Create a new logger if one isn't in the context or type assertion fails
	return NewLogger(LevelInfo)
}

// createErrorResponse creates a standardized error response
func createErrorResponse(message string) *protocol.CallToolResponse {
	return &protocol.CallToolResponse{
		IsError: true,
		Content: []protocol.ToolContent{
			{
				Type: "text",
				Text: message,
			},
		},
	}
}

// CallTool implements the ToolHandler interface for DeepseekServer
func (s *DeepseekServer) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	switch req.Name {
	case "deepseek_ask":
		return s.handleAskDeepseek(ctx, req)
	case "deepseek_models":
		return s.handleDeepseekModels(ctx)
	default:
		return createErrorResponse(fmt.Sprintf("unknown tool: %s", req.Name)), nil
	}
}

// handleAskDeepseek handles requests to the ask_deepseek tool
func (s *DeepseekServer) handleAskDeepseek(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	logger := getLoggerFromContext(ctx)

	// Extract and validate query parameter (required)
	query, ok := req.Arguments["query"].(string)
	if !ok {
		return createErrorResponse("query must be a string"), nil
	}

	// Extract optional model parameter
	modelName := s.config.DeepseekModel
	if customModel, ok := req.Arguments["model"].(string); ok && customModel != "" {
		// Validate the custom model
		if err := ValidateModelID(customModel); err != nil {
			logger.Error("Invalid model requested: %v", err)
			return createErrorResponse(fmt.Sprintf("Invalid model specified: %v", err)), nil
		}
		logger.Info("Using request-specific model: %s", customModel)
		modelName = customModel
	}

	// Extract optional systemPrompt parameter
	systemPrompt := s.config.DeepseekSystemPrompt
	if customPrompt, ok := req.Arguments["systemPrompt"].(string); ok && customPrompt != "" {
		logger.Info("Using request-specific system prompt")
		systemPrompt = customPrompt
	}

	// Extract file paths if provided
	var filePaths []string
	if filePathsRaw, ok := req.Arguments["file_paths"].([]interface{}); ok {
		for _, pathRaw := range filePathsRaw {
			if path, ok := pathRaw.(string); ok {
				filePaths = append(filePaths, path)
			}
		}
	}

	// Check if caching is requested
	useCache := false
	if useCacheRaw, ok := req.Arguments["use_cache"].(bool); ok {
		useCache = useCacheRaw
	}

	// Extract cache TTL if provided
	cacheTTL := ""
	if ttl, ok := req.Arguments["cache_ttl"].(string); ok {
		cacheTTL = ttl
	}

	// If caching is requested and the model supports it, use caching
	var cacheID string
	var cacheErr error
	if useCache && s.config.EnableCaching {
		// Check if model supports caching
		model := GetModelByID(modelName)
		if model != nil && model.SupportsCaching {
			// Create a cache
			cache, err := s.createCache(ctx, &CacheRequest{
				Model:        modelName,
				SystemPrompt: systemPrompt,
				FilePaths:    filePaths,
				TTL:          cacheTTL,
			})
			
			if err != nil {
				// Log the error but continue without caching
				logger.Warn("Failed to create cache, falling back to regular request: %v", err)
				cacheErr = err
			} else {
				cacheID = cache.ID
				logger.Info("Created cache with ID: %s", cacheID)
			}
		} else {
			// Model doesn't support caching, log warning and continue
			logger.Warn("Model %s does not support caching, falling back to regular request", modelName)
		}
	}

	// If we successfully created a cache, use it
	if cacheID != "" {
		return s.handleQueryWithCache(ctx, &protocol.CallToolRequest{
			Arguments: map[string]interface{}{
				"cache_id": cacheID,
				"query":    query,
			},
		})
	}

	// If caching failed or wasn't requested, use regular API
	chatMessages := []deepseek.ChatCompletionMessage{
		{
			Role:    deepseek.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    deepseek.ChatMessageRoleUser,
			Content: query,
		},
	}

	// Create the request
	request := &deepseek.ChatCompletionRequest{
		Model:       modelName,
		Messages:    chatMessages,
		Temperature: s.config.DeepseekTemperature,
	}

	// Log the temperature setting
	logger.Debug("Using temperature: %v for model %s", s.config.DeepseekTemperature, modelName)

	// Add file contents if provided
	if len(filePaths) > 0 {
		// First, gather file contents to be included in the prompt
		fileContents := "\n\n# Reference Files\n"
		successfulFiles := 0
		fileSizes := []int64{}
		
		for _, filePath := range filePaths {
			// Read file content using our readFile function
			content, err := readFile(filePath)
			if err != nil {
				logger.Error("Failed to read file %s: %v", filePath, err)
				continue
			}
			
			// Record successful file read and size
			successfulFiles++
			fileSizes = append(fileSizes, int64(len(content)))
			
			// Get language extension for markdown highlighting
			language := getLanguageFromPath(filePath)
			
			// Add file content to the combined contents with file name as header and proper markdown formatting
			fileContents += fmt.Sprintf("\n\n## %s\n\n```%s\n%s\n```", 
				filepath.Base(filePath), language, string(content))
		}
		
		// Log some statistics about the files
		logger.Info("Including %d file(s) in the query, total size: %s", 
			successfulFiles, humanReadableSize(sumSizes(fileSizes)))
		
		// Create a chat request with file contents embedded in the query
		if successfulFiles > 0 {
			query = query + fileContents
		} else {
			logger.Warn("No files were successfully read to include in the query")
		}
	}
	
	// Update the request with the full query (either original or with file contents)
	request.Messages[1].Content = query
	
	// Send the request to the DeepSeek API
	response, err := s.client.CreateChatCompletion(ctx, request)
	if err != nil {
		logger.Error("DeepSeek API error: %v", err)
		errorMsg := fmt.Sprintf("Error from DeepSeek API: %v", err)
		
		// Include additional information in the error response
		if len(filePaths) > 0 {
			errorMsg += fmt.Sprintf("\n\nThe request included %d file(s).", len(filePaths))
		}
		
		if cacheErr != nil {
			// Include cache error in response if it exists
			errorMsg += fmt.Sprintf("\n\nCache error: %v", cacheErr)
		}
		
		return createErrorResponse(errorMsg), nil
	}
	
	return s.formatResponse(response), nil
}

// handleQueryWithCache handles internal requests to query with a cached context
func (s *DeepseekServer) handleQueryWithCache(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	logger := getLoggerFromContext(ctx)
	logger.Info("Handling query with cache request")

	// Check if caching is enabled
	if !s.config.EnableCaching {
		return createErrorResponse("caching is disabled"), nil
	}

	// Extract and validate required parameters
	cacheID, ok := req.Arguments["cache_id"].(string)
	if !ok || cacheID == "" {
		return createErrorResponse("cache_id must be a non-empty string"), nil
	}

	query, ok := req.Arguments["query"].(string)
	if !ok || query == "" {
		return createErrorResponse("query must be a non-empty string"), nil
	}

	// Get cache info
	cacheInfo, err := s.getCache(ctx, cacheID)
	if err != nil {
		logger.Error("Failed to get cache info: %v", err)
		return createErrorResponse(fmt.Sprintf("failed to get cache: %v", err)), nil
	}

	// Get system prompt from cache
	systemPrompt := cacheInfo.SystemPrompt
	if systemPrompt == "" {
		// Fallback to default
		systemPrompt = s.config.DeepseekSystemPrompt
	}
	
	// Create messages array with system prompt
	messages := []deepseek.ChatCompletionMessage{
		{
			Role:    deepseek.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    deepseek.ChatMessageRoleUser,
			Content: query,
		},
	}
	
	// Add file contents if they were included in the cache
	if len(cacheInfo.FilePaths) > 0 {
		// Include the files in the message
		fileContents := "\n\n# Reference Files (from cached context)\n"
		successfulFiles := 0
		fileSizes := []int64{}
		
		for _, filePath := range cacheInfo.FilePaths {
			// Read file content using our readFile function
			content, err := readFile(filePath)
			if err != nil {
				logger.Error("Failed to read cached file %s: %v", filePath, err)
				continue
			}
			
			// Record successful file read and size
			successfulFiles++
			fileSizes = append(fileSizes, int64(len(content)))
			
			// Get language extension for markdown highlighting
			language := getLanguageFromPath(filePath)
			
			// Add file content to the combined contents with file name as header and proper markdown formatting
			fileContents += fmt.Sprintf("\n\n## %s\n\n```%s\n%s\n```", 
				filepath.Base(filePath), language, string(content))
		}
		
		// Log some statistics about the cached files
		logger.Info("Including %d cached file(s) in the query, total size: %s", 
			successfulFiles, humanReadableSize(sumSizes(fileSizes)))
		
		// Add file contents to the query if any files were successfully read
		if successfulFiles > 0 {
			messages[1].Content = query + fileContents
		}
	}
	
	request := &deepseek.ChatCompletionRequest{
		Model:       cacheInfo.Model,
		Messages:    messages,
		Temperature: s.config.DeepseekTemperature,
	}

	// Send the request
	response, err := s.client.CreateChatCompletion(ctx, request)
	if err != nil {
		logger.Error("DeepSeek API error in cached query: %v", err)
		
		// Create a detailed error message
		errorMsg := fmt.Sprintf("Error from DeepSeek API when using cached context: %v\n\n", err)
		errorMsg += fmt.Sprintf("Cache ID: %s\nModel: %s", cacheID, cacheInfo.Model)
		
		// Include file information if available
		if len(cacheInfo.FilePaths) > 0 {
			errorMsg += fmt.Sprintf("\nCache includes %d file path(s)", len(cacheInfo.FilePaths))
		}
		
		return createErrorResponse(errorMsg), nil
	}

	return s.formatResponse(response), nil
}

// handleDeepseekModels handles requests to the deepseek_models tool
func (s *DeepseekServer) handleDeepseekModels(ctx context.Context) (*protocol.CallToolResponse, error) {
	logger := getLoggerFromContext(ctx)
	logger.Info("Listing available DeepSeek models")

	// Get available models
	models := GetAvailableDeepseekModels()

	// Create a formatted response using strings.Builder with error handling
	var formattedContent strings.Builder

	// Define a helper function to write with error checking
	writeStringf := func(format string, args ...interface{}) error {
		_, err := formattedContent.WriteString(fmt.Sprintf(format, args...))
		return err
	}

	// Write the header
	if err := writeStringf("# Available DeepSeek Models\n\n"); err != nil {
		logger.Error("Error writing to response: %v", err)
		return createErrorResponse("Error generating model list"), nil
	}

	// Write each model's information
	for _, model := range models {
		if err := writeStringf("## %s\n", model.Name); err != nil {
			logger.Error("Error writing to response: %v", err)
			return createErrorResponse("Error generating model list"), nil
		}

		if err := writeStringf("- ID: `%s`\n", model.ID); err != nil {
			logger.Error("Error writing to response: %v", err)
			return createErrorResponse("Error generating model list"), nil
		}

		if err := writeStringf("- Description: %s\n", model.Description); err != nil {
			logger.Error("Error writing to response: %v", err)
			return createErrorResponse("Error generating model list"), nil
		}

		// Add caching support info
		if err := writeStringf("- Supports Caching: %v\n\n", model.SupportsCaching); err != nil {
			logger.Error("Error writing to response: %v", err)
			return createErrorResponse("Error generating model list"), nil
		}
	}

	// Add usage hint
	if err := writeStringf("## Usage\n"); err != nil {
		logger.Error("Error writing to response: %v", err)
		return createErrorResponse("Error generating model list"), nil
	}

	if err := writeStringf("You can specify a model ID in the `model` parameter when using the `deepseek_ask` tool:\n"); err != nil {
		logger.Error("Error writing to response: %v", err)
		return createErrorResponse("Error generating model list"), nil
	}

	if err := writeStringf("```json\n{\n  \"query\": \"Your question here\",\n  \"model\": \"deepseek-chat-001\",\n  \"use_cache\": true\n}\n```\n"); err != nil {
		logger.Error("Error writing to response: %v", err)
		return createErrorResponse("Error generating model list"), nil
	}

	// Add info about caching
	if err := writeStringf("\n## Caching\n"); err != nil {
		logger.Error("Error writing to response: %v", err)
		return createErrorResponse("Error generating model list"), nil
	}

	if err := writeStringf("Only models with version suffixes (e.g., ending with `-001`) support caching.\n"); err != nil {
		logger.Error("Error writing to response: %v", err)
		return createErrorResponse("Error generating model list"), nil
	}

	if err := writeStringf("When using a cacheable model, you can enable caching with the `use_cache` parameter. This will create a temporary cache that automatically expires after 10 minutes by default. You can specify a custom TTL with the `cache_ttl` parameter.\n"); err != nil {
		logger.Error("Error writing to response: %v", err)
		return createErrorResponse("Error generating model list"), nil
	}

	return &protocol.CallToolResponse{
		Content: []protocol.ToolContent{
			{
				Type: "text",
				Text: formattedContent.String(),
			},
		},
	}, nil
}

// executeDeepseekRequest makes the request to the DeepSeek API with retry capability
func (s *DeepseekServer) executeDeepseekRequest(ctx context.Context, model string, query string) (*deepseek.ChatCompletionResponse, error) {
	logger := getLoggerFromContext(ctx)

	var response *deepseek.ChatCompletionResponse

	// Define the operation to retry
	operation := func() error {
		var err error
		// Set timeout context for the API call
		timeoutCtx, cancel := context.WithTimeout(ctx, s.config.HTTPTimeout)
		defer cancel()

		request := &deepseek.ChatCompletionRequest{
			Model: model,
			Messages: []deepseek.ChatCompletionMessage{
				{
					Role:    deepseek.ChatMessageRoleUser,
					Content: query,
				},
			},
			Temperature: s.config.DeepseekTemperature,
		}
		response, err = s.client.CreateChatCompletion(timeoutCtx, request)
		if err != nil {
			logger.Error("DeepSeek API error: %v", err)
			return err
		}

		return nil
	}

	// Execute the operation with retry logic
	err := RetryWithBackoff(
		ctx,
		s.config.MaxRetries,
		s.config.InitialBackoff,
		s.config.MaxBackoff,
		operation,
		IsRetryableError, // Using the IsRetryableError from retry.go
		logger,
	)

	if err != nil {
		return nil, err
	}

	return response, nil
}

// formatResponse formats the DeepSeek API response
func (s *DeepseekServer) formatResponse(resp *deepseek.ChatCompletionResponse) *protocol.CallToolResponse {
	// Extract text from the response
	var content string
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	// Check for empty content and provide a fallback message
	if content == "" {
		content = "The DeepSeek model returned an empty response. This might indicate that the model couldn't generate an appropriate response for your query. Please try rephrasing your question or providing more context."
	}

	return &protocol.CallToolResponse{
		Content: []protocol.ToolContent{
			{
				Type: "text",
				Text: content,
			},
		},
	}
}

// Helper function to read a file
// This is declared at package level so it can be used by other files in the package
func readFile(path string) ([]byte, error) {
	// Use os.ReadFile to read the file from the file system
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return content, nil
}

// Helper function to get MIME type from file path
func getMimeTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	case ".wav":
		return "audio/wav"
	case ".doc", ".docx":
		return "application/msword"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".ppt", ".pptx":
		return "application/vnd.ms-powerpoint"
	case ".zip":
		return "application/zip"
	case ".csv":
		return "text/csv"
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".java":
		return "text/x-java"
	case ".c", ".cpp", ".h", ".hpp":
		return "text/x-c"
	case ".rb":
		return "text/plain"
	case ".php":
		return "text/plain"
	case ".md":
		return "text/markdown"
	default:
		return "application/octet-stream"
	}
}

// getLanguageFromPath returns the language identifier for syntax highlighting based on file extension
func getLanguageFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".java":
		return "java"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".md":
		return "markdown"
	case ".c":
		return "c"
	case ".cpp", ".hpp":
		return "cpp"
	case ".h":
		return "c"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".ts":
		return "typescript"
	case ".sh", ".bash":
		return "bash"
	case ".sql":
		return "sql"
	case ".yaml", ".yml":
		return "yaml"
	case ".rs":
		return "rust"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".groovy":
		return "groovy"
	case ".pl":
		return "perl"
	case ".r":
		return "r"
	case ".m":
		return "matlab"
	case ".ps1":
		return "powershell"
	case ".cs":
		return "csharp"
	case ".fs":
		return "fsharp"
	case ".vb":
		return "vbnet"
	case ".dart":
		return "dart"
	case ".ex", ".exs":
		return "elixir"
	case ".erl":
		return "erlang"
	case ".hs":
		return "haskell"
	case ".lua":
		return "lua"
	case ".jl":
		return "julia"
	case ".clj":
		return "clojure"
	// Default to text for unknown file types
	default:
		return "text"
	}
}

// sumSizes calculates the sum of an array of sizes
func sumSizes(sizes []int64) int64 {
	var total int64 = 0
	for _, size := range sizes {
		total += size
	}
	return total
}

// humanReadableSize formats a size in bytes to a human-readable string
func humanReadableSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
