package proxy

import (
	"bytes"
	"encoding/json"
)

type UsageData struct {
	PromptTokens     int64
	CompletionTokens int64
}

type usageResponse struct {
	Usage *struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

type ollamaResponse struct {
	PromptEvalCount int64 `json:"prompt_eval_count"`
	EvalCount       int64 `json:"eval_count"`
}

type llamaCppResponse struct {
	TokensEvaluated int64 `json:"tokens_evaluated"`
	TokensPredicted int64 `json:"tokens_predicted"`
}

// ExtractUsage parses token usage from a complete JSON response body.
// Supports OpenAI-compatible, Ollama native, and llama.cpp/BitNet native formats.
func ExtractUsage(body []byte) UsageData {
	// Try OpenAI-compatible format first (LM Studio, llama.cpp /v1/, etc.)
	var r usageResponse
	if err := json.Unmarshal(body, &r); err == nil && r.Usage != nil {
		return UsageData{
			PromptTokens:     r.Usage.PromptTokens,
			CompletionTokens: r.Usage.CompletionTokens,
		}
	}

	// Try Ollama native format (prompt_eval_count, eval_count)
	var ollama ollamaResponse
	if err := json.Unmarshal(body, &ollama); err == nil && (ollama.PromptEvalCount > 0 || ollama.EvalCount > 0) {
		return UsageData{
			PromptTokens:     ollama.PromptEvalCount,
			CompletionTokens: ollama.EvalCount,
		}
	}

	// Try llama.cpp / BitNet native format (tokens_evaluated, tokens_predicted)
	var llama llamaCppResponse
	if err := json.Unmarshal(body, &llama); err == nil && (llama.TokensEvaluated > 0 || llama.TokensPredicted > 0) {
		return UsageData{
			PromptTokens:     llama.TokensEvaluated,
			CompletionTokens: llama.TokensPredicted,
		}
	}

	return UsageData{}
}

// extractUsageFromJSON tries all known response formats on a raw JSON payload.
func extractUsageFromJSON(payload []byte) (UsageData, bool) {
	// Try OpenAI-compatible format
	var r usageResponse
	if err := json.Unmarshal(payload, &r); err == nil && r.Usage != nil {
		return UsageData{
			PromptTokens:     r.Usage.PromptTokens,
			CompletionTokens: r.Usage.CompletionTokens,
		}, true
	}

	// Try Ollama native format
	var ollama ollamaResponse
	if err := json.Unmarshal(payload, &ollama); err == nil && (ollama.PromptEvalCount > 0 || ollama.EvalCount > 0) {
		return UsageData{
			PromptTokens:     ollama.PromptEvalCount,
			CompletionTokens: ollama.EvalCount,
		}, true
	}

	// Try llama.cpp / BitNet native format
	var llama llamaCppResponse
	if err := json.Unmarshal(payload, &llama); err == nil && (llama.TokensEvaluated > 0 || llama.TokensPredicted > 0) {
		return UsageData{
			PromptTokens:     llama.TokensEvaluated,
			CompletionTokens: llama.TokensPredicted,
		}, true
	}

	return UsageData{}, false
}

// ExtractUsageFromSSE parses token usage from a single SSE data line (e.g. "data: {...}")
// or a raw NDJSON line (Ollama, llama.cpp native).
func ExtractUsageFromSSE(line []byte) (UsageData, bool) {
	line = bytes.TrimSpace(line)

	// SSE format: strip "data: " prefix
	if bytes.HasPrefix(line, []byte("data: ")) {
		payload := line[6:]
		if bytes.Equal(payload, []byte("[DONE]")) {
			return UsageData{}, false
		}
		return extractUsageFromJSON(payload)
	}

	// Raw NDJSON: try parsing the line directly as JSON
	if len(line) > 0 && line[0] == '{' {
		return extractUsageFromJSON(line)
	}

	return UsageData{}, false
}
