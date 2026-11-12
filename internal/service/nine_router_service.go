package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/util/httpclient"
	"go.uber.org/zap"
)

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ToolDefinition struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function ToolCallFunction `json:"function"`
}

type NineRouterMessage struct {
	Role       string     `json:"role"` // "system", "user", "assistant", "tool"
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type NineRouterChatRequest struct {
	Model       string              `json:"model"`
	Messages    []NineRouterMessage `json:"messages"`
	Tools       []ToolDefinition    `json:"tools,omitempty"`
	ToolChoice  interface{}         `json:"tool_choice,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream"`
}

type NineRouterChatResponse struct {
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type NineRouterStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type NineRouterEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type NineRouterEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

type NineRouterErrorPayload struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type NineRouterModelItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type NineRouterModelsResponse struct {
	Object string                `json:"object"`
	Data   []NineRouterModelItem `json:"data"`
}

type NineRouterService interface {
	IsConfigured() bool
	ListModels(ctx context.Context) ([]string, error)
	ChatCompletionWithModel(ctx context.Context, model string, messages []NineRouterMessage, tools []ToolDefinition) (*NineRouterChatResponse, error)
	StreamChatCompletionsWithModel(ctx context.Context, model string, messages []NineRouterMessage, onChunk func(delta string) error) error
	EmbedText(ctx context.Context, text string) ([]float32, error)
}

type nineRouterService struct {
	apiKey         string
	baseURL        string
	model          string
	embeddingModel string
	// allowedModels, when non-empty, restricts which upstream models are
	// offered to (and accepted from) clients (NINE_ROUTER_ALLOWED_MODELS).
	allowedModels map[string]struct{}
	httpClient    *http.Client
	logger        *zap.Logger
	mu            sync.RWMutex
	cachedModels  []string
	cacheExpiry   time.Time
}

// Upstream timeouts. Requests are always bound to the caller's context (the
// HTTP request for /chat/stream), so a client disconnect cancels the upstream
// call. The http.Client has no global Timeout because that would also cut
// long streaming answers; instead each call kind gets its own deadline.
const (
	nineRouterDialTimeout           = 10 * time.Second
	nineRouterTLSHandshakeTimeout   = 10 * time.Second
	nineRouterResponseHeaderTimeout = 120 * time.Second // non-streaming tool-call completions return headers only when done
	nineRouterListModelsTimeout     = 15 * time.Second
	nineRouterEmbedTimeout          = 30 * time.Second
	nineRouterCompletionTimeout     = 150 * time.Second
	nineRouterStreamTimeout         = 5 * time.Minute
)

func newNineRouterTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   nineRouterDialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   nineRouterTLSHandshakeTimeout,
		ResponseHeaderTimeout: nineRouterResponseHeaderTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   10,
		ForceAttemptHTTP2:     true,
	}
}

func NewNineRouterService(cfg *config.Config, logger *zap.Logger) NineRouterService {
	baseURL := cfg.NineRouterURL
	baseURL = strings.TrimRight(baseURL, "/") + "/"
	if baseURL == "/" {
		baseURL = "http://localhost:20128/v1/"
	}

	apiKey := cfg.NineRouterAPIKey
	if apiKey == "" {
		apiKey = "9router-local"
	}

	modelName := cfg.NineRouterModel

	embeddingModel := cfg.NineRouterEmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "embedding-3"
	}

	var allowedModels map[string]struct{}
	if len(cfg.NineRouterAllowedModels) > 0 {
		allowedModels = make(map[string]struct{}, len(cfg.NineRouterAllowedModels))
		for _, m := range cfg.NineRouterAllowedModels {
			allowedModels[m] = struct{}{}
		}
	}

	return &nineRouterService{
		apiKey:         apiKey,
		baseURL:        baseURL,
		model:          modelName,
		embeddingModel: embeddingModel,
		allowedModels:  allowedModels,
		httpClient: &http.Client{
			Transport: newLoggingTransport(logger),
		},
		logger: logger,
	}
}

func newLoggingTransport(logger *zap.Logger) http.RoundTripper {
	lt := httpclient.NewLoggingTransport("9Router-Gateway", logger)
	lt.Transport = newNineRouterTransport()
	return lt
}

func (s *nineRouterService) IsConfigured() bool {
	return s.baseURL != ""
}

func (s *nineRouterService) ListModels(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	if len(s.cachedModels) > 0 && time.Now().Before(s.cacheExpiry) {
		cached := make([]string, len(s.cachedModels))
		copy(cached, s.cachedModels)
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, nineRouterListModelsTimeout)
	defer cancel()

	httpReq, err := s.newRequest(ctx, http.MethodGet, "models", nil)
	if err != nil {
		return nil, err
	}
	var modelsResp NineRouterModelsResponse
	err = s.doJSON(httpReq, "failed to fetch models from 9Router", func(status int, body []byte) error {
		return fmt.Errorf("9Router returned status %d: %s", status, string(body))
	}, &modelsResp, "failed to decode models response")
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		if m.ID == "" {
			continue
		}
		if s.allowedModels != nil {
			if _, ok := s.allowedModels[m.ID]; !ok {
				continue
			}
		}
		result = append(result, m.ID)
	}

	s.mu.Lock()
	s.cachedModels = result
	s.cacheExpiry = time.Now().Add(60 * time.Second)
	s.mu.Unlock()

	// A copy, like a cache hit: callers must not be able to change the cache.
	return slices.Clone(result), nil
}

// isAllowedModel reports whether a client-requested model is one the server
// offers: the configured NINE_ROUTER_MODEL or an ID returned by ListModels
// (which is already narrowed to NINE_ROUTER_ALLOWED_MODELS when set).
// Arbitrary client strings are never forwarded upstream.
func (s *nineRouterService) isAllowedModel(ctx context.Context, requestedModel string) bool {
	if requestedModel == s.model {
		return true
	}
	models, err := s.ListModels(ctx)
	if err != nil {
		return false
	}
	for _, m := range models {
		if m == requestedModel {
			return true
		}
	}
	return false
}

func (s *nineRouterService) resolveModel(ctx context.Context, requestedModel string) string {
	if requestedModel != "" && requestedModel != "auto" && requestedModel != "default" {
		if s.isAllowedModel(ctx, requestedModel) {
			return requestedModel
		}
		s.logger.Warn("ignoring model not offered by the server", zap.String("requested_model", requestedModel))
	}
	if s.model != "" && s.model != "auto" && s.model != "default" {
		return s.model
	}

	// Dynamically discover available active model from 9Router providers
	models, err := s.ListModels(ctx)
	if err == nil && len(models) > 0 {
		s.logger.Info("dynamically selected active model from 9Router", zap.String("model", models[0]))
		return models[0]
	}

	return "default"
}

func formatNineRouterError(statusCode int, body []byte) string {
	var errPayload NineRouterErrorPayload
	if err := json.Unmarshal(body, &errPayload); err == nil && errPayload.Error.Message != "" {
		switch errPayload.Error.Code {
		case "1302", "rate_limit_exceeded", "429":
			return fmt.Sprintf("9Router Rate Limit (%s): %s", errPayload.Error.Code, errPayload.Error.Message)
		case "1113", "insufficient_quota":
			return fmt.Sprintf("9Router Quota (%s): %s", errPayload.Error.Code, errPayload.Error.Message)
		case "1211", "model_not_found":
			return fmt.Sprintf("9Router Model (%s): %s", errPayload.Error.Code, errPayload.Error.Message)
		case "1000", "1001", "1002", "1004", "invalid_api_key":
			return fmt.Sprintf("9Router Auth (%s): %s", errPayload.Error.Code, errPayload.Error.Message)
		default:
			return fmt.Sprintf("9Router Error (%s): %s", errPayload.Error.Code, errPayload.Error.Message)
		}
	}
	return fmt.Sprintf("HTTP %d: %s", statusCode, string(body))
}

// newRequest builds a request to path under the 9Router base URL with the
// bearer key. A non-nil payload is sent as a JSON body.
func (s *nineRouterService) newRequest(ctx context.Context, method, path string, payload interface{}) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal 9Router request: %w", err)
		}
		body = bytes.NewReader(b)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	if payload != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if s.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	return httpReq, nil
}

// do sends httpReq. A transport error is returned as "<failMsg>: err"; a
// non-200 answer is read, closed and turned into an error by onStatus. On
// success the caller must close resp.Body.
func (s *nineRouterService) do(httpReq *http.Request, failMsg string, onStatus func(status int, body []byte) error) (*http.Response, error) {
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", failMsg, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, onStatus(resp.StatusCode, respBody)
	}
	return resp, nil
}

// doJSON is do followed by decoding the JSON body into out; a decode failure
// is returned as "<decodeMsg>: err".
func (s *nineRouterService) doJSON(httpReq *http.Request, failMsg string, onStatus func(status int, body []byte) error, out interface{}, decodeMsg string) error {
	resp, err := s.do(httpReq, failMsg, onStatus)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: %w", decodeMsg, err)
	}
	return nil
}

// upstreamError logs a non-200 chat answer and returns its readable message
// (see formatNineRouterError).
func (s *nineRouterService) upstreamError(status int, body []byte) error {
	formattedErr := formatNineRouterError(status, body)
	s.logger.Error("9Router API error response",
		zap.Int("status", status),
		zap.String("body", string(body)),
		zap.String("detail", formattedErr),
	)
	return errors.New(formattedErr)
}

func (s *nineRouterService) ChatCompletionWithModel(ctx context.Context, model string, messages []NineRouterMessage, tools []ToolDefinition) (*NineRouterChatResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, nineRouterCompletionTimeout)
	defer cancel()

	chatReq := NineRouterChatRequest{
		Model:       s.resolveModel(ctx, model),
		Messages:    messages,
		Tools:       tools,
		Temperature: 0.2,
		Stream:      false,
	}
	if len(tools) > 0 {
		chatReq.ToolChoice = "auto"
	}

	httpReq, err := s.newRequest(ctx, http.MethodPost, "chat/completions", chatReq)
	if err != nil {
		return nil, err
	}
	var chatResp NineRouterChatResponse
	if err := s.doJSON(httpReq, "9Router API request failed", s.upstreamError, &chatResp, "failed to decode 9Router chat response"); err != nil {
		return nil, err
	}
	return &chatResp, nil
}

func (s *nineRouterService) StreamChatCompletionsWithModel(ctx context.Context, model string, messages []NineRouterMessage, onChunk func(delta string) error) error {
	ctx, cancel := context.WithTimeout(ctx, nineRouterStreamTimeout)
	defer cancel()

	chatReq := NineRouterChatRequest{
		Model:       s.resolveModel(ctx, model),
		Messages:    messages,
		Temperature: 0.4,
		Stream:      true,
	}

	httpReq, err := s.newRequest(ctx, http.MethodPost, "chat/completions", chatReq)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := s.do(httpReq, "9Router API request failed", s.upstreamError)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data: ") {
			dataContent := strings.TrimPrefix(trimmed, "data: ")
			if dataContent == "[DONE]" {
				break
			}
			if dataContent == "" {
				continue
			}

			var chunk NineRouterStreamChunk
			if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				if err := onChunk(chunk.Choices[0].Delta.Content); err != nil {
					return err
				}
			}
		}
	}

	return scanner.Err()
}

func (s *nineRouterService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, nineRouterEmbedTimeout)
	defer cancel()

	httpReq, err := s.newRequest(ctx, http.MethodPost, "embeddings", NineRouterEmbedRequest{
		Model: s.embeddingModel,
		Input: text,
	})
	if err != nil {
		return nil, err
	}
	var embedResp NineRouterEmbedResponse
	err = s.doJSON(httpReq, "9Router embedding request failed", func(status int, body []byte) error {
		return fmt.Errorf("9Router embedding error: %s", formatNineRouterError(status, body))
	}, &embedResp, "failed to decode 9Router embedding response")
	if err != nil {
		return nil, err
	}

	if len(embedResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned by 9Router")
	}

	return embedResp.Data[0].Embedding, nil
}
