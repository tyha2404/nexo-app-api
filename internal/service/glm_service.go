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

type GLMMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

type GLMChatRequest struct {
	Model       string       `json:"model"`
	Messages    []GLMMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	Stream      bool         `json:"stream"`
}

type GLMStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type GLMEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type GLMEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

type GLMErrorPayload struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type GLMService interface {
	IsConfigured() bool
	StreamChatCompletions(ctx context.Context, messages []GLMMessage, onChunk func(delta string) error) error
	EmbedText(ctx context.Context, text string) ([]float32, error)
}

type glmService struct {
	apiKey         string
	baseURL        string
	model          string
	embeddingModel string
	httpClient     *http.Client
	logger         *zap.Logger
}

func NewGLMService(cfg *config.Config, logger *zap.Logger) GLMService {
	baseURL := strings.TrimRight(cfg.GlmBaseURL, "/") + "/"
	if baseURL == "/" {
		baseURL = "https://open.bigmodel.cn/api/paas/v4/"
	}

	modelName := cfg.GlmModel
	if modelName == "" {
		modelName = "glm-4.7-flash"
	}

	embeddingModel := cfg.GlmEmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "embedding-3"
	}

	return &glmService{
		apiKey:         cfg.GlmApiKey,
		baseURL:        baseURL,
		model:          modelName,
		embeddingModel: embeddingModel,
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: httpclient.NewLoggingTransport("GLM-AI", logger),
		},
		logger: logger,
	}
}

func (s *glmService) IsConfigured() bool {
	return s.apiKey != ""
}

func formatGLMError(statusCode int, body []byte) string {
	var errPayload GLMErrorPayload
	if err := json.Unmarshal(body, &errPayload); err == nil && errPayload.Error.Message != "" {
		switch errPayload.Error.Code {
		case "1302":
			return fmt.Sprintf("GLM Rate Limit (Code %s): Tài khoản đã đạt giới hạn tần suất gọi API. Chi tiết: %s", errPayload.Error.Code, errPayload.Error.Message)
		case "1113":
			return fmt.Sprintf("GLM Quota (Code %s): Tài khoản không đủ số dư hoặc gói tài nguyên đã hết hạn. Chi tiết: %s", errPayload.Error.Code, errPayload.Error.Message)
		case "1211":
			return fmt.Sprintf("GLM Model (Code %s): Model không tồn tại hoặc không được hỗ trợ bởi API key này. Chi tiết: %s", errPayload.Error.Code, errPayload.Error.Message)
		case "1000", "1001", "1002", "1004":
			return fmt.Sprintf("GLM Auth (Code %s): API Key không hợp lệ hoặc đã hết hạn. Chi tiết: %s", errPayload.Error.Code, errPayload.Error.Message)
		default:
			return fmt.Sprintf("GLM Error (Code %s): %s", errPayload.Error.Code, errPayload.Error.Message)
		}
	}
	return fmt.Sprintf("HTTP %d: %s", statusCode, string(body))
}

func (s *glmService) StreamChatCompletions(ctx context.Context, messages []GLMMessage, onChunk func(delta string) error) error {
	if !s.IsConfigured() {
		return fmt.Errorf("GLM_API_KEY is not configured")
	}

	url := s.baseURL + "chat/completions"
	chatReq := GLMChatRequest{
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
		return fmt.Errorf("glm api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		formattedErr := formatGLMError(resp.StatusCode, respBody)
		s.logger.Error("glm api error response",
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

			var chunk GLMStreamChunk
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

func (s *glmService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("GLM_API_KEY is not configured")
	}

	url := s.baseURL + "embeddings"
	embedReq := GLMEmbedRequest{
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
		return nil, fmt.Errorf("glm embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		formattedErr := formatGLMError(resp.StatusCode, respBody)
		return nil, fmt.Errorf("glm embedding error: %s", formattedErr)
	}

	var embedResp GLMEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if len(embedResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned by GLM")
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
