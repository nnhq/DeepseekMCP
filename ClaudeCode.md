# DeepseekMCP Project Architecture

This document provides the technical architecture and implementation guidelines for DeepseekMCP, a Multi-Channel Proxy server for the DeepSeek API.

## Project Overview

DeepseekMCP serves as a proxy server integrating with the DeepSeek API, following patterns similar to the existing GeminiMCP implementation. It provides a standardized interface for accessing DeepSeek's AI models through the MCP protocol, focusing on features such as advanced code review capabilities, file management, and context caching.

## Architecture Components

The project follows a modular architecture with these primary components:

1. **Core Server**: Handles MCP protocol, request routing, and tool registration
2. **DeepSeek Client**: Manages communication with the DeepSeek API
3. **File Management**: File upload and management services for providing context to DeepSeek models
4. **Caching System**: Optimizes performance by caching contexts and responses
5. **Configuration**: Centralized configuration management
6. **Logging & Error Handling**: Consistent logging and robust error management

### Component Relationships

```
┌─────────────────┐      ┌────────────────┐      ┌────────────────────┐
│                 │      │                │      │                    │
│  MCP Protocol   ├─────►│ DeepseekServer ├─────►│  DeepSeek API      │
│  (External)     │      │ (Handler)      │      │  (External)        │
│                 │      │                │      │                    │
└─────────────────┘      └────────┬───────┘      └────────────────────┘
                                  │
                                  │
                    ┌─────────────┼──────────────┐
                    │             │              │
                    ▼             ▼              ▼
          ┌─────────────┐ ┌─────────────┐ ┌────────────┐
          │             │ │             │ │            │
          │ FileStore   │ │ CacheStore  │ │ Config     │
          │             │ │             │ │            │
          └─────────────┘ └─────────────┘ └────────────┘
```

## Key Features

1. **Multi-Model Support**: Support for various DeepSeek models
2. **Code Review Focus**: Built-in system prompt for code analysis
3. **File Management**: Automatic handling and integration of file uploads
4. **Enhanced Caching**: Persistent contexts with user-defined TTL for repeated queries
5. **Error Handling**: Graceful degradation with structured error logging
6. **Retry Logic**: Automatic retries with configurable exponential backoff for API calls

## Implementation Pattern

The implementation follows these key patterns:

1. **Server Implementation**: Implementing the ToolHandler interface from the MCP protocol
2. **Configuration-Driven Design**: Using environment variables and command-line flags for flexibility
3. **Middleware Pattern**: Using middleware for cross-cutting concerns like logging
4. **Repository Pattern**: Abstracting data access for files and caches
5. **Error State Handling**: Falling back to a degraded mode on critical errors

## API Tools

The server will provide these key tools:

1. **deepseek_ask**: For code analysis, review, and general queries with file context
2. **deepseek_models**: List available DeepSeek models with capabilities
3. **additional tools**: Consider adding specialized tools if DeepSeek offers unique capabilities

## Core Structures

### DeepseekServer

```go
type DeepseekServer struct {
    config     *Config
    client     *deepseek.Client
    fileStore  *FileStore
    cacheStore *CacheStore
}
```

### FileInfo & CacheInfo

```go
type FileInfo struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    URI         string    `json:"uri"`
    DisplayName string    `json:"display_name"`
    MimeType    string    `json:"mime_type"`
    Size        int64     `json:"size"`
    UploadedAt  time.Time `json:"uploaded_at"`
    ExpiresAt   time.Time `json:"expires_at"`
}

type CacheInfo struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    DisplayName string    `json:"display_name"`
    Model       string    `json:"model"`
    CreatedAt   time.Time `json:"created_at"`
    ExpiresAt   time.Time `json:"expires_at"`
    FileIDs     []string  `json:"file_ids,omitempty"`
}
```

## Implementation Steps

1. **Setup Project Structure**: Create base files and directory structure
2. **Core Configuration**: Implement configuration loading from environment
3. **Server Implementation**: Create DeepseekServer with MCP protocol support
4. **Client Integration**: Integrate DeepSeek Go client library
5. **File Management**: Implement file upload/download functionality
6. **Caching System**: Build caching layer for contexts
7. **API Tools**: Implement core tools (deepseek_ask, deepseek_models)
8. **Error Handling**: Set up comprehensive error handling and logging
9. **Testing & Documentation**: Add tests and documentation

## Key Differences from GeminiMCP

1. **API Differences**: Adapting to DeepSeek-specific API behavior and endpoints
2. **Model Support**: Supporting DeepSeek's model lineup instead of Gemini models
3. **Feature Mapping**: Mapping Gemini features to equivalent DeepSeek capabilities
4. **Authentication**: Using DeepSeek-specific authentication mechanisms

## Initial File Implementation Plan

1. `main.go`: Entry point, server initialization, command-line handling
2. `config.go`: Configuration loading and management
3. `deepseek.go`: DeepSeek API integration and request handling
4. `files.go`: File management system
5. `cache.go`: Caching system implementation
6. `models.go`: DeepSeek model definitions and management
7. `logger.go`: Logging utilities
8. `retry.go`: Retry mechanism for API calls
9. `middleware.go`: Request middleware implementation
10. `errors.go`: Error handling and degraded mode support

## Notes on DeepSeek API

Based on analysis of the deepseek-go library:

1. The client initialization is similar to Gemini but with DeepSeek-specific endpoints
2. DeepSeek models have different names and capabilities
3. The core content generation API is similar but with DeepSeek-specific parameters
4. DeepSeek supports file handling through its API
5. Need to support context and caching similar to GeminiMCP but adapted for DeepSeek

## Environment Variables

The server will be configurable through these environment variables:

```
DEEPSEEK_API_KEY=your_api_key
DEEPSEEK_MODEL=deepseek-chat
DEEPSEEK_SYSTEM_PROMPT=Your custom code review prompt
DEEPSEEK_MAX_FILE_SIZE=10485760  # 10MB
DEEPSEEK_ALLOWED_FILE_TYPES=text/x-go,text/markdown
DEEPSEEK_TIMEOUT=90
DEEPSEEK_MAX_RETRIES=2
DEEPSEEK_TEMPERATURE=0.4
DEEPSEEK_ENABLE_CACHING=true
DEEPSEEK_DEFAULT_CACHE_TTL=1h
```

## Next Steps

1. Initialize the Go module and dependencies
2. Implement the configuration loading system
3. Build the basic server structure with MCP protocol support
4. Integrate the DeepSeek Go client
5. Implement the core tools starting with deepseek_ask
