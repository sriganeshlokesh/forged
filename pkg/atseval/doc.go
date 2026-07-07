// Package atseval scores a structured resume against a job description
// using any OpenAI-compatible chat-completions endpoint (Groq, the
// Hugging Face router, a local Ollama, ...). The rubric and prompt design
// are adapted from HackerRank's MIT-licensed hiring-agent project,
// reworked into a job-description-match ("ATS") evaluation.
//
// The package is self-contained: it depends only on the Go standard
// library and defines its own input and output types, so it can be
// extracted into a standalone module unchanged.
//
// Usage:
//
//	ev := atseval.New(atseval.Options{
//		BaseURL: "https://api.groq.com/openai/v1",
//		APIKey:  os.Getenv("LLM_API_KEY"),
//		Model:   "llama-3.3-70b-versatile",
//	})
//	result, err := ev.Evaluate(ctx, jobDescription, atseval.Resume{
//		Summary: "<p>Backend engineer...</p>",
//		Experience: []atseval.Experience{{Company: "Acme", Role: "SWE"}},
//	})
package atseval
