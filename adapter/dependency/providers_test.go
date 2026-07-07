package dependency

import (
	"log/slog"
	"testing"

	evstub "github.com/sriganeshlokesh/forged/adapter/evaluator/stub"
	llmats "github.com/sriganeshlokesh/forged/adapter/llm/atseval"
	"github.com/sriganeshlokesh/forged/config"
)

func TestProvideEvaluator(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		wantStub bool
	}{
		{"no API key selects stub", "", true},
		{"API key selects LLM adapter", "gsk_test", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				LLMBaseURL: "https://api.groq.com/openai/v1",
				LLMAPIKey:  tt.apiKey,
				LLMModel:   "llama-3.3-70b-versatile",
			}
			got := ProvideEvaluator(cfg, slog.Default())

			_, isStub := got.(*evstub.StubEvaluator)
			_, isLLM := got.(*llmats.Adapter)
			if tt.wantStub && !isStub {
				t.Errorf("expected stub evaluator, got %T", got)
			}
			if !tt.wantStub && !isLLM {
				t.Errorf("expected LLM adapter, got %T", got)
			}
		})
	}
}
