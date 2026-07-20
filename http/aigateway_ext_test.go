package http

import (
	"encoding/json"
	"testing"
)

func TestExtractChatEnhancements(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4",
		"stream": true,
		"messages": [{"role":"user","content":"hi"}],
		"session_id": "sess-123",
		"prompt_template_id": "tpl-1",
		"prompt_vars": {"name": "Alice", "topic": "weather"},
		"moderate": true,
		"moderate_output": false,
		"cache_bypass": true
	}`)

	cleanBody, enh, model, stream, err := extractChatEnhancements(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %s", model)
	}
	if !stream {
		t.Fatal("expected stream true")
	}
	if enh.SessionID != "sess-123" {
		t.Fatalf("expected session_id sess-123, got %s", enh.SessionID)
	}
	if enh.PromptTemplateID != "tpl-1" {
		t.Fatalf("expected prompt_template_id tpl-1, got %s", enh.PromptTemplateID)
	}
	if enh.PromptVars["name"] != "Alice" || enh.PromptVars["topic"] != "weather" {
		t.Fatalf("unexpected prompt_vars: %v", enh.PromptVars)
	}
	if !enh.Moderate {
		t.Fatal("expected moderate true")
	}
	if enh.ModerateOutput {
		t.Fatal("expected moderate_output false")
	}
	if !enh.CacheBypass {
		t.Fatal("expected cache_bypass true")
	}

	// 增强字段应从 cleanBody 中移除
	var check map[string]any
	_ = json.Unmarshal(cleanBody, &check)
	if _, ok := check["session_id"]; ok {
		t.Fatal("session_id should be removed from cleanBody")
	}
	if _, ok := check["prompt_template_id"]; ok {
		t.Fatal("prompt_template_id should be removed from cleanBody")
	}
	if _, ok := check["cache_bypass"]; ok {
		t.Fatal("cache_bypass should be removed from cleanBody")
	}
	if _, ok := check["model"]; !ok {
		t.Fatal("model should remain in cleanBody")
	}
}

func TestExtractChatEnhancements_NoEnhancements(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)
	_, enh, model, stream, err := extractChatEnhancements(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %s", model)
	}
	if stream {
		t.Fatal("expected stream false")
	}
	if enh.SessionID != "" || enh.PromptTemplateID != "" {
		t.Fatal("expected empty enhancements")
	}
}

func TestExtractChatEnhancements_InvalidJSON(t *testing.T) {
	body := []byte(`{invalid json}`)
	_, _, _, _, err := extractChatEnhancements(body)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractLastUserMessage(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"system","content":"you are helpful"},
		{"role":"user","content":"what is 2+2"},
		{"role":"assistant","content":"4"},
		{"role":"user","content":"thanks"}
	]}`)
	msg := extractLastUserMessage(body)
	if msg != "thanks" {
		t.Fatalf("expected 'thanks', got %q", msg)
	}
}

func TestExtractLastUserMessage_NoUser(t *testing.T) {
	body := []byte(`{"messages":[{"role":"system","content":"hi"}]}`)
	msg := extractLastUserMessage(body)
	if msg != "" {
		t.Fatalf("expected empty, got %q", msg)
	}
}

func TestExtractAssistantContent(t *testing.T) {
	body := []byte(`{
		"choices": [{"message": {"content": "Hello there!"}}],
		"usage": {"completion_tokens": 3}
	}`)
	content := extractAssistantContent(body)
	if content != "Hello there!" {
		t.Fatalf("expected 'Hello there!', got %q", content)
	}
}

func TestExtractAssistantContent_NoChoices(t *testing.T) {
	body := []byte(`{"choices":[]}`)
	content := extractAssistantContent(body)
	if content != "" {
		t.Fatalf("expected empty, got %q", content)
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("hello", 10); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
	if got := truncateStr("hello world", 5); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}
