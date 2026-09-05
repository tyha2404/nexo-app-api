package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

func TestNineRouterService_Configured(t *testing.T) {
	cfg := &config.Config{
		NineRouterURL:    "http://localhost:20128/v1/",
		NineRouterAPIKey: "9router-local",
		NineRouterModel:  "gemini-2.5-flash",
	}
	logger := zap.NewNop()
	svc := service.NewNineRouterService(cfg, logger)

	assert.True(t, svc.IsConfigured(), "9Router local endpoint should be considered configured")
}

func TestNineRouterService_ChatCompletion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer 9router-local", r.Header.Get("Authorization"))

		resp := service.NineRouterChatResponse{}
		resp.Choices = append(resp.Choices, struct {
			Message struct {
				Role      string                 `json:"role"`
				Content   string                 `json:"content"`
				ToolCalls []service.ToolCall     `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			Message: struct {
				Role      string                 `json:"role"`
				Content   string                 `json:"content"`
				ToolCalls []service.ToolCall     `json:"tool_calls"`
			}{
				Role:    "assistant",
				Content: "Phản hồi từ 9Router AI Gateway!",
			},
			FinishReason: "stop",
		})

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		NineRouterURL:    server.URL + "/v1/",
		NineRouterAPIKey: "9router-local",
		NineRouterModel:  "gemini-2.5-flash",
	}
	logger := zap.NewNop()
	svc := service.NewNineRouterService(cfg, logger)

	res, err := svc.ChatCompletion(context.Background(), []service.NineRouterMessage{
		{Role: "user", Content: "Chào 9Router"},
	}, nil)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Len(t, res.Choices, 1)
	assert.Equal(t, "Phản hồi từ 9Router AI Gateway!", res.Choices[0].Message.Content)
}

func TestNineRouterService_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)

		resp := service.NineRouterModelsResponse{
			Object: "list",
			Data: []service.NineRouterModelItem{
				{ID: "gemini-2.5-flash", Object: "model", OwnedBy: "google"},
				{ID: "llama-3.3-70b-versatile", Object: "model", OwnedBy: "groq"},
				{ID: "deepseek/deepseek-chat", Object: "model", OwnedBy: "openrouter"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		NineRouterURL: server.URL + "/v1/",
	}
	logger := zap.NewNop()
	svc := service.NewNineRouterService(cfg, logger)

	models, err := svc.ListModels(context.Background())
	assert.NoError(t, err)
	assert.Len(t, models, 3)
	assert.Contains(t, models, "gemini-2.5-flash")
	assert.Contains(t, models, "llama-3.3-70b-versatile")
	assert.Contains(t, models, "deepseek/deepseek-chat")
}

func TestNineRouterService_ChatCompletionWithDynamicModel(t *testing.T) {
	var capturedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req service.NineRouterChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedModel = req.Model

		resp := service.NineRouterChatResponse{}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		NineRouterURL: server.URL + "/v1/",
	}
	logger := zap.NewNop()
	svc := service.NewNineRouterService(cfg, logger)

	// Call with dynamic model name
	_, _ = svc.ChatCompletionWithModel(context.Background(), "llama-3.3-70b-versatile", []service.NineRouterMessage{
		{Role: "user", Content: "Test"},
	}, nil)

	assert.Equal(t, "llama-3.3-70b-versatile", capturedModel)
}
