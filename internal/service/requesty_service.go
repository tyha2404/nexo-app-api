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

type RequestyMessage struct {
	Role       string     `json:"role"` // "system", "user", "assistant", "tool"
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type RequestyChatRequest struct {
	Model       string            `json:"model"`
	Messages    []RequestyMessage `json:"messages"`
	Tools       []ToolDefinition  `json:"tools,omitempty"`
	ToolChoice  interface{}       `json:"tool_choice,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	Stream      bool              `json:"stream"`
}

type RequestyChatResponse struct {
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type RequestyStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type RequestyEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type RequestyEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

type RequestyErrorPayload struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type RequestyService interface {
	IsConfigured() bool
	ChatCompletion(ctx context.Context, messages []RequestyMessage, tools []ToolDefinition) (*RequestyChatResponse, error)
	StreamChatCompletions(ctx context.Context, messages []RequestyMessage, onChunk func(delta string) error) error
	EmbedText(ctx context.Context, text string) ([]float32, error)
}

type requestyService struct {
	apiKey         string
	baseURL        string
	model          string
	embeddingModel string
	httpClient     *http.Client
	logger         *zap.Logger
}

func NewRequestyService(cfg *config.Config, logger *zap.Logger) RequestyService {
	baseURL := strings.TrimRight(cfg.RequestyBaseURL, "/") + "/"
	if baseURL == "/" {
		baseURL = "https://router.requesty.ai/v1/"
	}

	modelName := cfg.RequestyModel
	if modelName == "" {
		modelName = "google/gemma-4-31b-it"
	}

	embeddingModel := cfg.RequestyEmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "embedding-3"
	}

	return &requestyService{
		apiKey:         cfg.RequestyApiKey,
		baseURL:        baseURL,
		model:          modelName,
		embeddingModel: embeddingModel,
		httpClient: &http.Client{
			Timeout:   300 * time.Second,
			Transport: httpclient.NewLoggingTransport("Requesty-AI", logger),
		},
		logger: logger,
	}
}

func (s *requestyService) IsConfigured() bool {
	return s.apiKey != ""
}

func formatRequestyError(statusCode int, body []byte) string {
	var errPayload RequestyErrorPayload
	if err := json.Unmarshal(body, &errPayload); err == nil && errPayload.Error.Message != "" {
		switch errPayload.Error.Code {
		case "1302", "rate_limit_exceeded":
			return fmt.Sprintf("Requesty Rate Limit (Code %s): Tài khoản đã đạt giới hạn tần suất gọi API. Chi tiết: %s", errPayload.Error.Code, errPayload.Error.Message)
		case "1113", "insufficient_quota":
			return fmt.Sprintf("Requesty Quota (Code %s): Tài khoản không đủ số dư hoặc gói tài nguyên đã hết hạn. Chi tiết: %s", errPayload.Error.Code, errPayload.Error.Message)
		case "1211", "model_not_found":
			return fmt.Sprintf("Requesty Model (Code %s): Model không tồn tại hoặc không được hỗ trợ bởi API key này. Chi tiết: %s", errPayload.Error.Code, errPayload.Error.Message)
		case "1000", "1001", "1002", "1004", "invalid_api_key":
			return fmt.Sprintf("Requesty Auth (Code %s): API Key không hợp lệ hoặc đã hết hạn. Chi tiết: %s", errPayload.Error.Code, errPayload.Error.Message)
		default:
			return fmt.Sprintf("Requesty Error (Code %s): %s", errPayload.Error.Code, errPayload.Error.Message)
		}
	}
	return fmt.Sprintf("HTTP %d: %s", statusCode, string(body))
}

func (s *requestyService) ChatCompletion(ctx context.Context, messages []RequestyMessage, tools []ToolDefinition) (*RequestyChatResponse, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("Requesty API Key is not configured")
	}

	url := s.baseURL + "chat/completions"
	chatReq := RequestyChatRequest{
		Model:       s.model,
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
		return nil, fmt.Errorf("failed to marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Requesty API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		formattedErr := formatRequestyError(resp.StatusCode, respBody)
		s.logger.Error("Requesty API error response",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
			zap.String("detail", formattedErr),
		)
		return nil, fmt.Errorf("%s", formattedErr)
	}

	var chatResp RequestyChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode chat response: %w", err)
	}

	return &chatResp, nil
}

func (s *requestyService) StreamChatCompletions(ctx context.Context, messages []RequestyMessage, onChunk func(delta string) error) error {
	if !s.IsConfigured() {
		return fmt.Errorf("Requesty API Key is not configured")
	}

	url := s.baseURL + "chat/completions"
	chatReq := RequestyChatRequest{
		Model:       s.model,
		Messages:    messages,
		Temperature: 0.4,
		Stream:      true,
	}

	bodyBytes, err := json.Marshal(chatReq)
	if err != nil {
		return fmt.Errorf("failed to marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("Requesty API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		formattedErr := formatRequestyError(resp.StatusCode, respBody)
		s.logger.Error("Requesty API error response",
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

			var chunk RequestyStreamChunk
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

func (s *requestyService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("Requesty API Key is not configured")
	}

	url := s.baseURL + "embeddings"
	embedReq := RequestyEmbedRequest{
		Model: s.embeddingModel,
		Input: text,
	}

	bodyBytes, err := json.Marshal(embedReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Requesty embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		formattedErr := formatRequestyError(resp.StatusCode, respBody)
		return nil, fmt.Errorf("Requesty embedding error: %s", formattedErr)
	}

	var embedResp RequestyEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if len(embedResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned by Requesty")
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
