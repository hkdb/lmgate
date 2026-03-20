package middleware

import (
	"testing"
)

func TestMatchPattern_WildcardAll(t *testing.T) {
	if !matchPattern("*", "anything") {
		t.Error("'*' should match anything")
	}
}

func TestMatchPattern_PrefixWildcard(t *testing.T) {
	if !matchPattern("llama3:*", "llama3:8b") {
		t.Error("'llama3:*' should match 'llama3:8b'")
	}
}

func TestMatchPattern_PrefixWildcard_NoMatch(t *testing.T) {
	if matchPattern("llama3:*", "mistral:7b") {
		t.Error("'llama3:*' should not match 'mistral:7b'")
	}
}

func TestMatchPattern_SuffixWildcard(t *testing.T) {
	if !matchPattern("*:latest", "llama3:latest") {
		t.Error("'*:latest' should match 'llama3:latest'")
	}
}

func TestMatchPattern_SuffixWildcard_NoMatch(t *testing.T) {
	if matchPattern("*:latest", "llama3:8b") {
		t.Error("'*:latest' should not match 'llama3:8b'")
	}
}

func TestMatchPattern_NoWildcard(t *testing.T) {
	if matchPattern("llama3:8b", "mistral:7b") {
		t.Error("exact pattern should not match different model")
	}
}
