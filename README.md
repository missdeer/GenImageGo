# GenImageGo

Generate images from text prompts using OpenAI-compatible APIs, Google Gemini, or Vertex AI. Includes a CLI and an optional web UI.

## Features

- Multi-provider support: OpenAI, Gemini, Vertex AI
- CLI with JSON config file support
- Image-to-image support (optional input images)
- Gemini/Vertex AI aspect ratio and resolution controls
- Automatic image format handling for API compatibility

## Requirements

- Go 1.24+

## Build

```bash
go build -o genImage.exe
```

```bash
go build ./...
```

## CLI Usage

```bash
./genImage.exe --prompt "A scenic lake" --output output.jpg
```

```bash
./genImage.exe --config config.json
```

```bash
./genImage.exe --api-service gemini --model gemini-3-pro-image-preview --prompt "A cute cat"
```

## Web Server Mode

```bash
go run . --serve --addr 127.0.0.1:8080
```

Open `http://127.0.0.1:8080` in a browser. Static assets are embedded at build time, so the `static/` directory is not required at runtime (the `--static` flag is ignored).

## Configuration Notes

- Use `--api-key` or set `api_key` in `config.json`.
- Vertex AI requires `--project`, and optionally `--location` and `--credentials`.

## Password Reset Email Configuration

To enable the "Forgot Password" feature, configure SMTP settings in your `config.json`:

```json
{
  "smtp": {
    "host": "smtp.example.com",
    "port": 587,
    "username": "your-email@example.com",
    "password": "your-password",
    "from": "noreply@example.com"
  },
  "base_web_url": "http://your-domain.com"
}
```

| Field | Description |
|-------|-------------|
| `host` | SMTP server hostname |
| `port` | SMTP port (587 for STARTTLS, 465 for SSL/TLS) |
| `username` | SMTP authentication username |
| `password` | SMTP authentication password |
| `from` | Sender email address (optional, defaults to username) |
| `base_web_url` | Base URL for password reset links (optional, defaults to server address) |

## License

This software is available under a dual licensing model:

- **Open Source (GPL v3):** For non-commercial, personal, educational, or open source projects.
- **Commercial License:** Required for workplace or commercial use.

See the [LICENSE](LICENSE) file for full details.
