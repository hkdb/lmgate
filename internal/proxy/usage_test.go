package proxy

import (
	"testing"
)

func TestExtractUsage_OpenAIFormat(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":10,"completion_tokens":20}}`)
	u := ExtractUsage(body)
	if u.PromptTokens != 10 || u.CompletionTokens != 20 {
		t.Errorf("got prompt=%d completion=%d, want 10/20", u.PromptTokens, u.CompletionTokens)
	}
}

func TestExtractUsage_OllamaFormat(t *testing.T) {
	body := []byte(`{"prompt_eval_count":15,"eval_count":25}`)
	u := ExtractUsage(body)
	if u.PromptTokens != 15 || u.CompletionTokens != 25 {
		t.Errorf("got prompt=%d completion=%d, want 15/25", u.PromptTokens, u.CompletionTokens)
	}
}

func TestExtractUsage_LlamaCppFormat(t *testing.T) {
	body := []byte(`{"tokens_evaluated":30,"tokens_predicted":40}`)
	u := ExtractUsage(body)
	if u.PromptTokens != 30 || u.CompletionTokens != 40 {
		t.Errorf("got prompt=%d completion=%d, want 30/40", u.PromptTokens, u.CompletionTokens)
	}
}

func TestExtractUsage_InvalidJSON(t *testing.T) {
	u := ExtractUsage([]byte(`not json`))
	if u.PromptTokens != 0 || u.CompletionTokens != 0 {
		t.Errorf("expected zero usage for invalid JSON, got %+v", u)
	}
}

func TestExtractUsage_EmptyBody(t *testing.T) {
	u := ExtractUsage([]byte{})
	if u.PromptTokens != 0 || u.CompletionTokens != 0 {
		t.Errorf("expected zero usage for empty body, got %+v", u)
	}
}

func TestExtractUsageFromSSE_DataPrefix(t *testing.T) {
	line := []byte(`data: {"usage":{"prompt_tokens":5,"completion_tokens":10}}`)
	u, ok := ExtractUsageFromSSE(line)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if u.PromptTokens != 5 || u.CompletionTokens != 10 {
		t.Errorf("got prompt=%d completion=%d, want 5/10", u.PromptTokens, u.CompletionTokens)
	}
}

func TestExtractUsageFromSSE_DoneMessage(t *testing.T) {
	line := []byte(`data: [DONE]`)
	_, ok := ExtractUsageFromSSE(line)
	if ok {
		t.Fatal("expected ok=false for [DONE]")
	}
}

func TestExtractUsageFromSSE_RawNDJSON(t *testing.T) {
	line := []byte(`{"prompt_eval_count":7,"eval_count":14}`)
	u, ok := ExtractUsageFromSSE(line)
	if !ok {
		t.Fatal("expected ok=true for NDJSON")
	}
	if u.PromptTokens != 7 || u.CompletionTokens != 14 {
		t.Errorf("got prompt=%d completion=%d, want 7/14", u.PromptTokens, u.CompletionTokens)
	}
}

func TestExtractUsageFromSSE_EmptyLine(t *testing.T) {
	_, ok := ExtractUsageFromSSE([]byte(""))
	if ok {
		t.Fatal("expected ok=false for empty line")
	}
}
