# CLAUDE.md — Architecture law for forged

`forged` is the resume-builder backend (Go); future iterations add LLM/cloud-API resume optimization. Architecture follows [RanchoCooper/go-hexagonal](https://github.com/RanchoCooper/go-hexagonal) with chi on net/http, wire DI, slog JSON logging, and Railway deployment. This file is authoritative: all code changes must comply with the rules below.

## Commands

```bash
make all          # fmt + lint + test + build (run before every commit)
make run          # go run ./cmd (local dev)
make wire         # regenerate wire_gen.go after any edit to adapter/dependency/wire.go
make mocks        # regenerate mockery mocks after any change to a consumer interface
make test         # go test -race -coverprofile=coverage.out ./...
make lint         # golangci-lint run ./...
make docker-build # build forged:local image
make tidy         # go mod tidy
```

After editing `adapter/dependency/wire.go`, always run `make wire` and commit `wire_gen.go` together.

## Architecture: the dependency rule is law

Strict inward-only imports. No exceptions except where noted.

| Layer       | Path           | May import                                   | Must NEVER import            |
|-------------|----------------|----------------------------------------------|------------------------------|
| domain      | `domain/`      | stdlib, other `domain/*`                     | application, adapter, api, config, any third-party lib |
| application | `application/` | domain, stdlib                               | adapter, api                 |
| adapter     | `adapter/`     | application, domain, config, infra libraries | api (sole exception: `adapter/dependency`) |
| api         | `api/`         | application, domain, config, chi             | adapter                      |
| cmd         | `cmd/`         | config, util, adapter/dependency             | business logic (stays thin)  |

`adapter/dependency` is the composition root — the **only** place that may import both adapter and api packages simultaneously.

`config` and `util` are importable everywhere **except** domain (domain stays pure, zero third-party dependencies forever).

`ctx` (`context.Context`) flows through every layer boundary; never store it in structs.

## Where new code goes

### New endpoint
1. Handler: `api/http/handle/<feature>.go`
2. Route: add to `api/http/router.go`
3. DTOs: `api/dto/<feature>.go`
4. Use case: `application/<feature>/` implementing `core.UseCase`
5. Wire provider: add to `adapter/dependency/wire.go`, then `make wire`

### New domain concept
- Entity + sentinel errors: `domain/model/`
- Domain logic: `domain/service/`

### New external integration (database, LLM, HuggingFace, etc.)
1. The consumer (use case, handler) declares the interface it needs in its own constructor file (see Dependency injection below)
2. Implementation in `adapter/<kind>/` (e.g. `adapter/llm/atseval/`) — satisfies the consumer interface implicitly
3. Wiring in `adapter/dependency/`: `wire.Bind` consumer interface → impl, or a `Provide*` function in `providers.go` when selection depends on config
4. API code **never** touches adapter packages directly
5. Add the new interface to `.mockery.yaml`, run `make mocks`, use the mock in unit tests

### New config value
1. Add field to `Config` struct in `config/config.go`
2. Read env var with default in `Load()`
3. Document in README env-var table

## Conventions

- **Dependency injection**: every constructor takes its dependencies as **interfaces, never concrete types**. The interface is declared **in the same file as the constructor that consumes it** and contains **only the methods that package actually uses** (consumer-defined interfaces, e.g. `evaluation.ResumeEvaluator`, `handle.EvaluationUseCase`, `apihttp.HealthRoutes`). Implementations satisfy them implicitly; `adapter/dependency` binds them with `wire.Bind` or a provider function
- **Interface naming**: name for the behavior the consumer needs (`ResumeEvaluator`, `EvaluationUseCase`) — no I-prefix
- **Mocks**: generated with **mockery** (`.mockery.yaml` at repo root, output in `<pkg>/mocks/`). Workflow: add/change a consumer interface → register it in `.mockery.yaml` → `make mocks` → commit the generated files. Unit tests use generated mocks (`mocks.NewMockX(t)` + `.EXPECT()`), never hand-rolled fakes
- **Package names**: lowercase, single word; `error_code` grandfathered for reference-repo parity
- **Error codes**: 10000+ API · 20000+ auth · 30000+ internal · 40000+ business (see `api/error_code/`)
- **Logging**: `log/slog` only — never `fmt.Println`, never `log.Printf`. Every log line includes `service` and `env` from the default logger
- **Wire workflow**: edit `wire.go` → `make wire` → commit **both** `wire.go` and `wire_gen.go`
- **Tests**: table-driven where multiple cases apply; `httptest.NewRecorder` for HTTP handlers; mockery mocks for injected dependencies
- **Commits**: Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`)

## Deployment (Railway)

- Build method: **DOCKERFILE** (see `railway.toml`)
- Port: Railway injects `$PORT`; server binds `":"+PORT` (dual-stack wildcard — covers IPv4 + IPv6; Railway healthchecks use IPv6 private networking)
- `/health` gates deploy **Active** status — it must always return 200 unconditionally; never add dependencies to this handler
- Railway sends **SIGTERM** on redeploy; the server calls `Shutdown` with `SHUTDOWN_TIMEOUT` and then exits cleanly

## Hard rules

1. **Never violate the import table** — if a linter can enforce it, wire it up
2. **Domain stays third-party-free permanently** — no external imports ever
3. **Every use case implements `core.UseCase`** (`application/core/interfaces.go`)
4. **`wire_gen.go` is always committed and current** — CI enforces freshness with `git diff --exit-code`
5. **`/health` stays dependency-free and always returns 200** — a dead DB must never block a Railway deploy
6. **`PORT` bind uses `":"+PORT`** — never `"0.0.0.0:"`
