package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gomcpgo/mcp/pkg/protocol"
)

// Implements the ListTools method of ToolHandler
func (m *LoggerMiddleware) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	if handler, ok := m.handler.(interface {
		ListTools(ctx context.Context) (*protocol.ListToolsResponse, error)
	}); ok {
		// Add logger to context
		ctx = context.WithValue(ctx, loggerKey, m.logger)
		
		// Track execution time
		start := time.Now()
		
		// Log request
		m.logger.Info("ListTools called")
		
		// Execute the handler
		resp, err := handler.ListTools(ctx)
		
		// Log completion and execution time
		m.execTime = time.Since(start)
		if err != nil {
			m.logger.Error("ListTools failed: %v (took %v)", err, m.execTime)
		} else {
			m.logger.Info("ListTools completed successfully with %d tools (took %v)", 
				len(resp.Tools), m.execTime)
		}
		
		return resp, err
	}
	
	return nil, fmt.Errorf("handler does not implement ToolHandler")
}

// Implements the CallTool method of ToolHandler
func (m *LoggerMiddleware) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	if handler, ok := m.handler.(interface {
		CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error)
	}); ok {
		// Add logger to context
		ctx = context.WithValue(ctx, loggerKey, m.logger)
		
		// Track execution time
		start := time.Now()
		
		// Log request details
		if req.Arguments != nil {
			if query, ok := req.Arguments["query"].(string); ok && len(query) > 100 {
				// Truncate long queries for readability
				m.logger.Info("CallTool: %s (query: %s...)", req.Name, query[:100])
			} else {
				m.logger.Info("CallTool: %s", req.Name)
			}
		} else {
			m.logger.Info("CallTool: %s (no arguments)", req.Name)
		}
		
		// Execute the handler
		resp, err := handler.CallTool(ctx, req)
		
		// Log completion and execution time
		m.execTime = time.Since(start)
		if err != nil {
			m.logger.Error("CallTool %s failed: %v (took %v)", req.Name, err, m.execTime)
		} else {
			m.logger.Info("CallTool %s completed successfully (took %v)", req.Name, m.execTime)
		}
		
		return resp, err
	}
	
	return nil, fmt.Errorf("handler does not implement ToolHandler")
}

// ErrorDeepseekServer is a minimal implementation used when the main server fails to initialize
type ErrorDeepseekServer struct {
	errorMessage string
	config       *Config
}

// ListTools implements the ToolHandler interface for the error server
func (s *ErrorDeepseekServer) ListTools(ctx context.Context) (*protocol.ListToolsResponse, error) {
	tools := []protocol.Tool{
		{
			Name:        "deepseek_error",
			Description: "Reports the error that prevented normal initialization",
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

// CallTool implements the ToolHandler interface for the error server
func (s *ErrorDeepseekServer) CallTool(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResponse, error) {
	// Always return an error message with initialized state
	errorMessage := s.errorMessage
	if errorMessage == "" {
		errorMessage = "The server is running in degraded mode due to an unknown error during initialization"
	}

	var configInfo string
	if s.config != nil {
		// Include some minimal config info if available
		configInfo = fmt.Sprintf("\n\nServer configuration (partial):\n- Model: %s\n- Caching: %v",
			s.config.DeepseekModel, s.config.EnableCaching)
	}

	return &protocol.CallToolResponse{
		Content: []protocol.ToolContent{
			{
				Type: "text",
				Text: fmt.Sprintf("# DeepseekMCP Server Error\n\n%s%s\n\nPlease check server logs for more details or correct the configuration and restart the server.", errorMessage, configInfo),
			},
		},
	}, nil
}
