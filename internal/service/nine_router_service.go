package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
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

// RequestyMessage alias for backward compatibility
type RequestyMessage = NineRouterMessage

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

// RequestyChatResponse alias for backward compatibility
type RequestyChatResponse = NineRouterChatResponse

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
	ChatCompletion(ctx context.Context, messages []NineRouterMessage, tools []ToolDefinition) (*NineRouterChatResponse, error)
	ChatCompletionWithModel(ctx context.Context, model string, messages []NineRouterMessage, tools []ToolDefinition) (*NineRouterChatResponse, error)
	StreamChatCompletions(ctx context.Context, messages []NineRouterMessage, onChunk func(delta string) error) error
	StreamChatCompletionsWithModel(ctx context.Context, model string, messages []NineRouterMessage, onChunk func(delta string) error) error
	EmbedText(ctx context.Context, text string) ([]float32, error)
}

// RequestyService alias for backward compatibility
type RequestyService = NineRouterService

type nineRouterService struct {
	apiKey         string
	baseURL        string
	model          string
	embeddingModel string
	httpClient     *http.Client
	logger         *zap.Logger
	mu             sync.RWMutex
	cachedModels   []string
	cacheExpiry    time.Time
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

	return &nineRouterService{
		apiKey:         apiKey,
		baseURL:        baseURL,
		model:          modelName,
		embeddingModel: embeddingModel,
		httpClient: &http.Client{
			Timeout:   300 * time.Second,
			Transport: httpclient.NewLoggingTransport("9Router-Gateway", logger),
		},
		logger: logger,
	}
}

// NewRequestyService constructor alias
func NewRequestyService(cfg *config.Config, logger *zap.Logger) NineRouterService {
	return NewNineRouterService(cfg, logger)
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

	url := s.baseURL + "models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create models request: %w", err)
	}

	if s.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from 9Router: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("9Router returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var modelsResp NineRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	result := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		if m.ID != "" {
			result = append(result, m.ID)
		}
	}

	s.mu.Lock()
	s.cachedModels = result
	s.cacheExpiry = time.Now().Add(60 * time.Second)
	s.mu.Unlock()

	return result, nil
}

func (s *nineRouterService) resolveModel(ctx context.Context, requestedModel string) string {
	if requestedModel != "" && requestedModel != "auto" && requestedModel != "default" {
		return requestedModel
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

func (s *nineRouterService) ChatCompletion(ctx context.Context, messages []NineRouterMessage, tools []ToolDefinition) (*NineRouterChatResponse, error) {
	return s.ChatCompletionWithModel(ctx, "", messages, tools)
}

func (s *nineRouterService) ChatCompletionWithModel(ctx context.Context, model string, messages []NineRouterMessage, tools []ToolDefinition) (*NineRouterChatResponse, error) {
	url := s.baseURL + "chat/completions"
	effectiveModel := s.resolveModel(ctx, model)

	chatReq := NineRouterChatRequest{
		Model:       effectiveModel,
		Messages:    messages,
		Tools:       tools,
		Temperature: 0.2,
		Stream:      false,
	}
	if len(tools) > 0 {
		chatReq.ToolChoice = "auto"
	}

	bodyBytes, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal 9Router chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("9Router API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		formattedErr := formatNineRouterError(resp.StatusCode, respBody)
		s.logger.Error("9Router API error response",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
			zap.String("detail", formattedErr),
		)
		return nil, fmt.Errorf("%s", formattedErr)
	}

	var chatResp NineRouterChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode 9Router chat response: %w", err)
	}

	return &chatResp, nil
}

func (s *nineRouterService) StreamChatCompletions(ctx context.Context, messages []NineRouterMessage, onChunk func(delta string) error) error {
	return s.StreamChatCompletionsWithModel(ctx, "", messages, onChunk)
}

func (s *nineRouterService) StreamChatCompletionsWithModel(ctx context.Context, model string, messages []NineRouterMessage, onChunk func(delta string) error) error {
	url := s.baseURL + "chat/completions"
	effectiveModel := s.resolveModel(ctx, model)

	chatReq := NineRouterChatRequest{
		Model:       effectiveModel,
		Messages:    messages,
		Temperature: 0.4,
		Stream:      true,
	}

	bodyBytes, err := json.Marshal(chatReq)
	if err != nil {
		return fmt.Errorf("failed to marshal 9Router chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("9Router API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		formattedErr := formatNineRouterError(resp.StatusCode, respBody)
		s.logger.Error("9Router API error response",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
			zap.String("detail", formattedErr),
		)
		return fmt.Errorf("%s", formattedErr)
	}

	scanner := bufio.NewScanner(resp.Body)
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
	url := s.baseURL + "embeddings"
	embedReq := NineRouterEmbedRequest{
		Model: s.embeddingModel,
		Input: text,
	}

	bodyBytes, err := json.Marshal(embedReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal 9Router embedding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("9Router embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		formattedErr := formatNineRouterError(resp.StatusCode, respBody)
		return nil, fmt.Errorf("9Router embedding error: %s", formattedErr)
	}

	var embedResp NineRouterEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode 9Router embedding response: %w", err)
	}

	if len(embedResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned by 9Router")
	}

	return embedResp.Data[0].Embedding, nil
}

// ComputeCosineSimilarity calculates cosine similarity between two float vectors
func ComputeCosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct float32
	var normA float32
	var normB float32

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
