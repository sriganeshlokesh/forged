# forged

A hexagonal Go backend for the resume-builder webapp. Exposes a `/health` endpoint and is deployable to Railway via Docker.

## Quickstart

Go 1.26 is required. If you have Go 1.22+, `GOTOOLCHAIN=auto` will download it automatically.

```bash
make run
curl -s localhost:8080/health
# {"status":"ok","service":"forged","version":"dev"}
```

## Make targets

| Target | Description |
|---|---|
| `make all` | fmt + lint + test + build |
| `make fmt` | gofmt -s -w |
| `make lint` | golangci-lint run |
| `make test` | go test -race + coverage |
| `make build` | CGO_ENABLED=0 build → bin/forged |
| `make run` | go run ./cmd |
| `make wire` | Regenerate wire_gen.go |
| `make docker-build` | Build forged:local Docker image |
| `make tidy` | go mod tidy |

## Configuration (env vars)

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `APP_ENV` | `development` | Environment name (development, staging, production) |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `SERVICE_NAME` | `forged` | Service name added to every log line |
| `HTTP_READ_TIMEOUT` | `10s` | HTTP server read timeout |
| `HTTP_WRITE_TIMEOUT` | `30s` | HTTP server write timeout |
| `HTTP_IDLE_TIMEOUT` | `120s` | HTTP server idle connection timeout |
| `SHUTDOWN_TIMEOUT` | `5s` | Graceful shutdown timeout after SIGTERM |
| `LLM_BASE_URL` | `https://api.groq.com/openai/v1` | OpenAI-compatible chat-completions endpoint (Groq, Hugging Face router, local Ollama) |
| `LLM_API_KEY` | *(empty)* | API key for the LLM endpoint; when empty the stub evaluator is used |
| `LLM_MODEL` | `llama-3.3-70b-versatile` | Model name at the LLM endpoint |
| `LLM_TIMEOUT` | `60s` | Per-request timeout for LLM calls |
| `RATE_LIMIT_PER_IP_RPM` | `10` | Max requests/minute per client IP on `/v1/evaluations` (`0` disables) |
| `RATE_LIMIT_REVISIONS_PER_IP_RPM` | `20` | Max requests/minute per client IP on `/v1/revisions` (`0` disables) |
| `CORS_ALLOWED_ORIGINS` | *(empty)* | Comma-separated browser origins allowed to call the API cross-origin, e.g. `https://drafted.up.railway.app`. Empty disables CORS. |

## Docker

```bash
make docker-build
docker run --rm -p 8080:8080 -e PORT=8080 forged:local
```

## Railway deployment

**Via CLI:**
```bash
brew install railway
railway login
railway init
railway up
railway domain
```

**Via GitHub:** Connect your repository in the Railway dashboard for push-to-deploy. The `/health` endpoint gates the deployment Active status — it must return 200 for a deploy to succeed.

See [CLAUDE.md](./CLAUDE.md) for architecture details.
