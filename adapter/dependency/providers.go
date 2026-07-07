package dependency

import (
	"log/slog"

	evstub "github.com/sriganeshlokesh/forged/adapter/evaluator/stub"
	llmats "github.com/sriganeshlokesh/forged/adapter/llm/atseval"
	"github.com/sriganeshlokesh/forged/config"
	"github.com/sriganeshlokesh/forged/domain/service"
	"github.com/sriganeshlokesh/forged/pkg/atseval"
)

// ProvideEvaluator selects the resume evaluator implementation at startup:
// the LLM-backed engine when an API key is configured, otherwise the stub
// so local dev and deploys work without credentials.
func ProvideEvaluator(cfg *config.Config, logger *slog.Logger) service.IResumeEvaluator {
	if cfg.LLMAPIKey == "" {
		logger.Warn("LLM_API_KEY not set — using stub evaluator")
		return evstub.NewStubEvaluator()
	}
	engine := atseval.New(atseval.Options{
		BaseURL: cfg.LLMBaseURL,
		APIKey:  cfg.LLMAPIKey,
		Model:   cfg.LLMModel,
		Timeout: cfg.LLMTimeout,
	})
	logger.Info("using LLM evaluator",
		slog.String("base_url", cfg.LLMBaseURL),
		slog.String("model", cfg.LLMModel),
	)
	return llmats.New(engine, logger)
}
