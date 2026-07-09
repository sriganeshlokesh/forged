//go:build wireinject
// +build wireinject

// Package dependency is the composition root for the application.
// It is the ONLY adapter package allowed to import api/*.
//
// Workflow: edit wire.go → make wire → commit wire_gen.go.
// Never import this package from application or domain layers.
package dependency

import (
	"log/slog"
	"net/http"

	"github.com/google/wire"

	apihttp "github.com/sriganeshlokesh/forged/api/http"
	"github.com/sriganeshlokesh/forged/api/http/handle"
	"github.com/sriganeshlokesh/forged/application/evaluation"
	"github.com/sriganeshlokesh/forged/application/revision"
	"github.com/sriganeshlokesh/forged/config"
)

// ServerSet groups the providers needed to build an *http.Server.
// Consumer-declared interfaces are bound to their implementations here.
var ServerSet = wire.NewSet(
	handle.NewHealthHandler,
	handle.NewEvaluationHandler,
	handle.NewRevisionHandler,
	evaluation.NewUseCase,
	revision.NewUseCase,
	ProvideEvaluator,
	ProvideReviser,
	apihttp.NewRouter,
	apihttp.NewServer,
	wire.Bind(new(handle.EvaluationUseCase), new(*evaluation.UseCase)),
	wire.Bind(new(handle.RevisionUseCase), new(*revision.UseCase)),
	wire.Bind(new(apihttp.HealthRoutes), new(*handle.HealthHandler)),
	wire.Bind(new(apihttp.EvaluationRoutes), new(*handle.EvaluationHandler)),
	wire.Bind(new(apihttp.RevisionRoutes), new(*handle.RevisionHandler)),
)

// InitializeServer is the wire injector that builds the complete *http.Server.
// wire.Build is replaced by generated code in wire_gen.go.
func InitializeServer(cfg *config.Config, logger *slog.Logger) *http.Server {
	wire.Build(ServerSet)
	return nil
}
