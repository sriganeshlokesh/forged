package dependency

import (
	"log/slog"

	evstub "github.com/sriganeshlokesh/forged/adapter/evaluator/stub"
	llmats "github.com/sriganeshlokesh/forged/adapter/llm/atseval"
	"github.com/sriganeshlokesh/forged/application/evaluation"
	"github.com/sriganeshlokesh/forged/application/revision"
	"github.com/sriganeshlokesh/forged/config"
	"github.com/sriganeshlokesh/forged/pkg/atseval"
)

// ProvideEvaluator selects the resume evaluator implementation at startup:
// the LLM-backed engine when an API key is configured, otherwise the stub
// so local dev and deploys work without credentials.
func ProvideEvaluator(cfg *config.Config, logger *slog.Logger) evaluation.ResumeEvaluator {
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

// ProvideReviser selects the resume reviser implementation at startup:
// the LLM-backed engine when an API key is configured, otherwise the
// deterministic stub so local dev and deploys work without credentials.
func ProvideReviser(cfg *config.Config, logger *slog.Logger) revision.ResumeReviser {
	if cfg.LLMAPIKey == "" {
		logger.Warn("LLM_API_KEY not set — using stub reviser")
		return evstub.NewStubReviser()
	}
	engine := atseval.New(atseval.Options{
		BaseURL: cfg.LLMBaseURL,
		APIKey:  cfg.LLMAPIKey,
		Model:   cfg.LLMModel,
		Timeout: cfg.LLMTimeout,
	})
	logger.Info("using LLM reviser",
		slog.String("base_url", cfg.LLMBaseURL),
		slog.String("model", cfg.LLMModel),
	)
	return llmats.New(engine, logger)
}
