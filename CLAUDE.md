# CLAUDE 

## Workflow

- **MUST** follow multi-agent-workflow
- carefully read and understand rules defined in `.claude/rules` directory and follow them strictly

## Build and Run Commands

```bash
# Build Windows binary
go build -o genImage.exe

# Verify module builds
go build ./...

# Run from source
go run . --prompt "A scenic lake" --output output.jpg

# Run with config file
./genImage.exe --config config.json

# Run tests (when added)
go test ./...
```

## Architecture

GenImageGo is a CLI tool for AI image generation supporting multiple providers:

- **main.go**: CLI entry point using `pflag`. Parses arguments, loads config, routes to appropriate client
- **config.go**: Config structures (`Config`, `GeminiConfig`, `OpenAIConfig`), JSON loading, defaults. Config priority: CLI > config file > defaults
- **gemini.go**: `GeminiClient` for Gemini/Vertex AI APIs. Builds multipart requests with text+images, parses base64 image responses
- **openai.go**: `OpenAIClient` for OpenAI-compatible APIs. Uses chat completions with vision, extracts images from markdown responses
- **utils.go**: File I/O, base64 encoding, BMP→PNG conversion, MIME type detection

## Key API Services

| Service | Flag Value | Notes |
|---------|------------|-------|
| Gemini | `gemini` | Default. Uses `/v1beta/models/{model}:generateContent` |
| Vertex AI | `vertexai` | Requires `--project`. Uses Google Cloud credentials |
| OpenAI | `openai` | Uses `/chat/completions`. Parses markdown `![image](data:...)` |

## Common CLI Flags

- `-s, --api-service`: `openai`, `gemini`, `vertexai`
- `-m, --model`: Model name
- `-p, --prompt` / `-f, --prompt-file`: Text prompt (mutually exclusive)
- `-o, --output`: Output image path
- `-t, --aspect-ratio`: For gemini/vertexai (e.g., `16:9`, `3:4`)
- `-r, --resolution`: For gemini/vertexai (`1K`, `2K`, `4K`)

## Code Style

- Standard Go formatting (`gofmt`)
- Error messages in Chinese (用户向错误信息使用中文)
- Wrap errors with `fmt.Errorf` and print to stderr