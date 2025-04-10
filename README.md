# DeepSeek MCP Server

A production-grade MCP server integrating with DeepSeek's API, featuring advanced code review capabilities, efficient file management, and sophisticated cached context handling.

## Features

- **Multi-Model Support**: Choose from various DeepSeek models including DeepSeek Chat and DeepSeek Coder
- **Code Review Focus**: Built-in system prompt for detailed code analysis with markdown output
- **Automatic File Handling**: Built-in file management with direct path integration
- **Enhanced Caching**: Create persistent contexts with user-defined TTL for repeated queries
- **Advanced Error Handling**: Graceful degradation with structured error logging
- **Improved Retry Logic**: Automatic retries with configurable exponential backoff for API calls
- **Security**: Configurable file type restrictions and size limits
- **Performance Monitoring**: Built-in metrics collection for request latency and throughput

## Prerequisites

- Go 1.21+
- DeepSeek API key
- Basic understanding of MCP protocol

## Installation & Quick Start

```bash
# Clone and build
git clone https://github.com/your-username/DeepseekMCP
cd DeepseekMCP
go build -o deepseek-mcp

# Start server with environment variables
export DEEPSEEK_API_KEY=your_api_key
export DEEPSEEK_MODEL=deepseek-chat
./deepseek-mcp
```

## Configuration

### Essential Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DEEPSEEK_API_KEY` | DeepSeek API key | *Required* |
| `DEEPSEEK_MODEL` | Model ID from `models.go` | `deepseek-chat` |
| `DEEPSEEK_SYSTEM_PROMPT` | System prompt for code review | *Custom review prompt* |
| `DEEPSEEK_MAX_FILE_SIZE` | Max upload size (bytes) | `10485760` (10MB) |
| `DEEPSEEK_ALLOWED_FILE_TYPES` | Comma-separated MIME types | [Common text/code types] |

### Optimization Variables
| Variable | Description | Default |
|----------|-------------|---------|
| `DEEPSEEK_TIMEOUT` | API timeout in seconds | `90` |
| `DEEPSEEK_MAX_RETRIES` | Max API retries | `2` |
| `DEEPSEEK_INITIAL_BACKOFF` | Initial backoff time (seconds) | `1` |
| `DEEPSEEK_MAX_BACKOFF` | Maximum backoff time (seconds) | `10` |
| `DEEPSEEK_TEMPERATURE` | Model temperature (0.0-1.0) | `0.4` |
| `DEEPSEEK_ENABLE_CACHING` | Enable context caching | `true` |
| `DEEPSEEK_DEFAULT_CACHE_TTL` | Default cache time-to-live | `1h` |

Example `.env`:
```env
DEEPSEEK_API_KEY=your_api_key
DEEPSEEK_MODEL=deepseek-chat
DEEPSEEK_SYSTEM_PROMPT="Your custom code review prompt here"
DEEPSEEK_MAX_FILE_SIZE=5242880  # 5MB
DEEPSEEK_ALLOWED_FILE_TYPES=text/x-go,text/markdown
```

## Core API Tools

Currently, the server provides two main tools:

### deepseek_ask

Used for code analysis, review, and general queries with optional file path inclusion and caching.

```json
{
  "name": "deepseek_ask",
  "arguments": {
    "query": "Review this Go code for concurrency issues...",
    "model": "deepseek-chat-001",
    "systemPrompt": "Optional custom review instructions",
    "file_paths": ["main.go", "config.go"],
    "use_cache": true,
    "cache_ttl": "1h"
  }
}
```

### deepseek_models

Lists all available DeepSeek models with their capabilities and caching support.

```json
{
  "name": "deepseek_models",
  "arguments": {}
}
```

## Supported Models

The following DeepSeek models are supported:

| Model ID | Description | Caching Support |
|----------|-------------|----------------|
| `deepseek-chat` | General-purpose chat model balancing performance and efficiency | No |
| `deepseek-coder` | Specialized model for coding and technical tasks | No |
| `deepseek-reasoner` | Model optimized for reasoning and problem-solving tasks | No |
| `deepseek-chat-001` | Stable version of DeepSeek Chat with version suffix | Yes |
| `deepseek-coder-001` | Stable version of DeepSeek Coder with version suffix | Yes |

## Supported File Types
| Extension | MIME Type | 
|-----------|-----------|
| .go       | text/x-go |
| .py       | text/x-python |
| .js       | text/javascript |
| .md       | text/markdown |
| .java     | text/x-java |
| .c/.h     | text/x-c |
| .cpp/.hpp | text/x-c++ |
| 25+ more  | (See `getMimeTypeFromPath` in deepseek.go) |

## Operational Notes

- **Degraded Mode**: Automatically enters safe mode on initialization errors
- **Audit Logging**: All operations logged with timestamps and metadata
- **Security**: File content validated by MIME type and size before processing

## File Handling

The server handles files directly through the `deepseek_ask` tool:

1. Specify local file paths in the `file_paths` array parameter
2. The server automatically:
   - Reads the files from the provided paths
   - Determines the correct MIME type based on file extension
   - Uploads the file content to the DeepSeek API
   - Uses the files as context for the query

This direct file handling approach eliminates the need for separate file upload/management endpoints.

## Caching Functionality

The server supports enhanced caching capabilities:

- **Automatic Caching**: Simply set `use_cache: true` in the `deepseek_ask` request
- **TTL Control**: Specify cache expiration with the `cache_ttl` parameter (e.g., "10m", "2h")
- **Model Support**: Only models with version suffixes (ending with `-001`) support caching
- **Context Persistence**: Uploaded files are automatically stored and associated with the cache

Example with caching:
```json
{
  "name": "deepseek_ask",
  "arguments": {
    "query": "Review this code considering the best practices we discussed earlier",
    "model": "deepseek-chat-001",
    "use_cache": true,
    "cache_ttl": "1h",
    "file_paths": ["main.go", "config.go"]
  }
}
```

## Development

### Running Tests

To run tests:

```bash
go test -v
```

### Running Linter

```bash
./run_lint.sh
```

### Formatting Code

```bash
./run_format.sh
```

## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the project
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
