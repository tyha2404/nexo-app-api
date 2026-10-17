package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
				Role      string             `json:"role"`
				Content   string             `json:"content"`
				ToolCalls []service.ToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			Message: struct {
				Role      string             `json:"role"`
				Content   string             `json:"content"`
				ToolCalls []service.ToolCall `json:"tool_calls"`
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
		if r.URL.Path == "/v1/models" {
			_ = json.NewEncoder(w).Encode(service.NineRouterModelsResponse{
				Object: "list",
				Data:   []service.NineRouterModelItem{{ID: "gemini-2.5-flash"}, {ID: "llama-3.3-70b-versatile"}},
			})
			return
		}
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

func TestNineRouterService_IgnoresModelNotOfferedByServer(t *testing.T) {
	var capturedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			_ = json.NewEncoder(w).Encode(service.NineRouterModelsResponse{
				Object: "list",
				Data:   []service.NineRouterModelItem{{ID: "gemini-2.5-flash"}},
			})
			return
		}
		var req service.NineRouterChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedModel = req.Model
		_ = json.NewEncoder(w).Encode(service.NineRouterChatResponse{})
	}))
	defer server.Close()

	svc := service.NewNineRouterService(&config.Config{
		NineRouterURL:   server.URL + "/v1/",
		NineRouterModel: "gemini-2.5-flash",
	}, zap.NewNop())

	_, _ = svc.ChatCompletionWithModel(context.Background(), "gpt-4o-expensive-unlisted", []service.NineRouterMessage{
		{Role: "user", Content: "Test"},
	}, nil)

	assert.Equal(t, "gemini-2.5-flash", capturedModel, "arbitrary client model must not be forwarded upstream")
}

func TestNineRouterService_StreamCancelledWithContext(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n"))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done() // hang until the client goes away
	}))
	defer server.Close()

	svc := service.NewNineRouterService(&config.Config{
		NineRouterURL:   server.URL + "/v1/",
		NineRouterModel: "gemini-2.5-flash",
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- svc.StreamChatCompletionsWithModel(ctx, "", []service.NineRouterMessage{{Role: "user", Content: "hi"}}, func(string) error { return nil })
	}()

	<-started
	cancel()
	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not stop after context cancellation")
	}
}
