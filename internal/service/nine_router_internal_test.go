package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"go.uber.org/zap"
)

func TestFormatNineRouterError(t *testing.T) {
	payload := func(code, msg string) []byte {
		return []byte(fmt.Sprintf(`{"error":{"code":%q,"message":%q}}`, code, msg))
	}
	tests := []struct {
		name   string
		status int
		body   []byte
		want   string
	}{
		{"rate limit", 429, payload("rate_limit_exceeded", "slow down"), "9Router Rate Limit (rate_limit_exceeded): slow down"},
		{"rate limit numeric", 429, payload("1302", "chậm lại"), "9Router Rate Limit (1302): chậm lại"},
		{"quota", 402, payload("insufficient_quota", "no credit"), "9Router Quota (insufficient_quota): no credit"},
		{"model", 404, payload("1211", "unknown model"), "9Router Model (1211): unknown model"},
		{"auth", 401, payload("invalid_api_key", "bad key"), "9Router Auth (invalid_api_key): bad key"},
		{"auth numeric", 401, payload("1002", "bad key"), "9Router Auth (1002): bad key"},
		{"other code", 500, payload("boom", "kaput"), "9Router Error (boom): kaput"},
		{"no message falls back to raw body", 502, payload("x", ""), `HTTP 502: {"error":{"code":"x","message":""}}`},
		{"non JSON body", 503, []byte("Service Unavailable"), "HTTP 503: Service Unavailable"},
		{"empty body", 500, nil, "HTTP 500: "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formatNineRouterError(tc.status, tc.body))
		})
	}
}

// modelsServer serves /models with the given IDs and counts the requests.
func modelsServer(t *testing.T, ids []string, status int, hits *int32, extra http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			atomic.AddInt32(hits, 1)
			if status != http.StatusOK {
				http.Error(w, "nope", status)
				return
			}
			resp := NineRouterModelsResponse{Object: "list"}
			for _, id := range ids {
				resp.Data = append(resp.Data, NineRouterModelItem{ID: id})
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if extra != nil {
			extra(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestNineRouter(baseURL, model string, allowed ...string) *nineRouterService {
	return NewNineRouterService(&config.Config{
		NineRouterURL: baseURL, NineRouterModel: model, NineRouterAllowedModels: allowed,
	}, zap.NewNop()).(*nineRouterService)
}

func TestResolveModel(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		configured string
		upstream   []string
		status     int
		requested  string
		want       string
		wantHits   int32
	}{
		{"configured model when none requested", "glm-4", []string{"a"}, 200, "", "glm-4", 0},
		{"auto means server default", "glm-4", []string{"a"}, 200, "auto", "glm-4", 0},
		{"requested equals configured needs no lookup", "glm-4", nil, 200, "glm-4", "glm-4", 0},
		{"requested model offered upstream", "glm-4", []string{"gpt-x", "claude"}, 200, "claude", "claude", 1},
		{"unknown requested model is ignored", "glm-4", []string{"gpt-x"}, 200, "evil-model", "glm-4", 1},
		{"no configured model picks first upstream", "", []string{"first", "second"}, 200, "", "first", 1},
		{"configured 'default' discovers upstream", "default", []string{"first"}, 200, "default", "first", 1},
		{"discovery failure falls back to 'default'", "", nil, 500, "", "default", 1},
		{"requested model with failing discovery is ignored", "", nil, 500, "gpt-x", "default", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var hits int32
			srv := modelsServer(t, tc.upstream, tc.status, &hits, nil)
			s := newTestNineRouter(srv.URL+"/v1", tc.configured)
			assert.Equal(t, tc.want, s.resolveModel(ctx, tc.requested))
			assert.Equal(t, tc.wantHits, atomic.LoadInt32(&hits))
		})
	}
}

func TestListModels_AllowListAndCache(t *testing.T) {
	var hits int32
	srv := modelsServer(t, []string{"a", "", "b", "c"}, 200, &hits, nil)
	s := newTestNineRouter(srv.URL+"/v1/", "a", "a", "c")

	models, err := s.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "c"}, models, "only allow-listed, non-empty IDs")

	again, err := s.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "c"}, again)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits), "second call served from cache")
	again[0] = "mutated"
	third, _ := s.ListModels(context.Background())
	assert.Equal(t, []string{"a", "c"}, third, "a cache hit returns a copy")

	assert.False(t, s.isAllowedModel(context.Background(), "b"), "upstream model outside the allow-list is rejected")

	var failHits int32
	bad := modelsServer(t, nil, http.StatusUnauthorized, &failHits, nil)
	_, err = newTestNineRouter(bad.URL, "").ListModels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestListModels_FirstResultDoesNotAliasCache(t *testing.T) {
	var hits int32
	srv := modelsServer(t, []string{"a", "b"}, 200, &hits, nil)
	s := newTestNineRouter(srv.URL, "")
	first, err := s.ListModels(context.Background())
	require.NoError(t, err)
	first[0] = "mutated"
	again, _ := s.ListModels(context.Background())
	assert.Equal(t, []string{"a", "b"}, again)
}

func sseHandler(t *testing.T, lines []string, gotBody *NineRouterChatRequest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		if gotBody != nil {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, gotBody)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, l := range lines {
			_, _ = io.WriteString(w, l+"\n")
		}
	}
}

func chunkLine(content string) string {
	return fmt.Sprintf(`data: {"choices":[{"delta":{"content":%q}}]}`, content)
}

func TestStreamChatCompletions_ParsesSSE(t *testing.T) {
	var hits int32
	var body NineRouterChatRequest
	srv := modelsServer(t, nil, 200, &hits, sseHandler(t, []string{
		": keep-alive comment",
		chunkLine("Xin "),
		"data: ",
		"data: {not json}",
		`data: {"choices":[]}`,
		chunkLine("chào"),
		"event: ping",
		"data: [DONE]",
		chunkLine("SAU DONE"),
	}, &body))
	s := newTestNineRouter(srv.URL+"/v1", "glm-4")

	var got []string
	err := s.StreamChatCompletions(context.Background(), []NineRouterMessage{{Role: "user", Content: "hi"}}, func(d string) error {
		got = append(got, d)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Xin ", "chào"}, got, "malformed/empty events skipped, stops at [DONE]")
	assert.True(t, body.Stream)
	assert.Equal(t, "glm-4", body.Model)
	assert.Empty(t, body.Tools, "the answer stream never offers tools")
}

func TestStreamChatCompletions_CallbackErrorStops(t *testing.T) {
	var hits int32
	srv := modelsServer(t, nil, 200, &hits, sseHandler(t, []string{chunkLine("a"), chunkLine("b"), chunkLine("c")}, nil))
	s := newTestNineRouter(srv.URL+"/v1", "glm-4")
	stop := errors.New("client gone")
	n := 0
	err := s.StreamChatCompletionsWithModel(context.Background(), "", nil, func(string) error {
		n++
		if n == 2 {
			return stop
		}
		return nil
	})
	require.ErrorIs(t, err, stop)
	assert.Equal(t, 2, n)
}

func TestStreamAndChat_ErrorStatusIsFormatted(t *testing.T) {
	var hits int32
	srv := modelsServer(t, nil, 200, &hits, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"code":"429","message":"quá tải"}}`)
	})
	s := newTestNineRouter(srv.URL+"/v1", "glm-4")

	err := s.StreamChatCompletionsWithModel(context.Background(), "", nil, func(string) error { return nil })
	require.Error(t, err)
	assert.Equal(t, "9Router Rate Limit (429): quá tải", err.Error())

	_, err = s.ChatCompletion(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Equal(t, "9Router Rate Limit (429): quá tải", err.Error())
}

func TestChatCompletion_SendsToolsAndDecodeError(t *testing.T) {
	var hits int32
	var body NineRouterChatRequest
	srv := modelsServer(t, nil, 200, &hits, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = io.WriteString(w, "not json")
	})
	s := newTestNineRouter(srv.URL+"/v1", "glm-4")

	_, err := s.ChatCompletionWithModel(context.Background(), "", []NineRouterMessage{{Role: "user", Content: "hi"}}, GetFinancialToolDefinitions()[:2])
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
	assert.Equal(t, "auto", body.ToolChoice)
	assert.Len(t, body.Tools, 2)
	assert.False(t, body.Stream)
}

func TestEmbedText(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		want    []float32
		wantErr string
	}{
		{"ok", 200, `{"data":[{"embedding":[0.1,0.2,0.3]}]}`, []float32{0.1, 0.2, 0.3}, ""},
		{"no data", 200, `{"data":[]}`, nil, "no embedding"},
		{"bad json", 200, `{`, nil, "decode"},
		{"upstream error", 401, `{"error":{"code":"invalid_api_key","message":"bad key"}}`, nil, "9Router Auth (invalid_api_key): bad key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hits int32
			var got NineRouterEmbedRequest
			srv := modelsServer(t, nil, 200, &hits, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/embeddings", r.URL.Path)
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &got)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			s := NewRequestyService(&config.Config{NineRouterURL: srv.URL + "/v1", NineRouterAPIKey: "k"}, zap.NewNop())

			vec, err := s.EmbedText(context.Background(), "tiết kiệm")
			assert.Equal(t, "tiết kiệm", got.Input)
			assert.Equal(t, "embedding-3", got.Model, "default embedding model")
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, vec)
		})
	}
}

func TestNewNineRouterService_Defaults(t *testing.T) {
	s := newTestNineRouter("", "")
	assert.Equal(t, "http://localhost:20128/v1/", s.baseURL)
	assert.Equal(t, "9router-local", s.apiKey)
	assert.Equal(t, "embedding-3", s.embeddingModel)
	assert.Nil(t, s.allowedModels)

	s = newTestNineRouter("http://gw/v1///", "m", "x")
	assert.Equal(t, "http://gw/v1/", s.baseURL)
	assert.Contains(t, s.allowedModels, "x")
}
