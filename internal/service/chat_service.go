package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"go.uber.org/zap"
)

type ChatService interface {
	CreateSession(ctx context.Context, userID uuid.UUID, title string) (*dto.ChatSessionResponse, error)
	ListSessions(ctx context.Context, userID uuid.UUID, limit int) ([]dto.ChatSessionResponse, error)
	GetSessionMessages(ctx context.Context, userID, sessionID uuid.UUID, limit int) (*dto.ChatSessionResponse, error)
	DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error
	ClearSessions(ctx context.Context, userID uuid.UUID) error
	ProcessMessageStream(ctx context.Context, userID uuid.UUID, req dto.SendMessageRequest, eventChan chan<- dto.ChatStreamEvent) error
}

type chatService struct {
	chatRepo   repository.ChatRepository
	glmService GLMService
	ragService RAGService
	logger     *zap.Logger
}

func NewChatService(
	chatRepo repository.ChatRepository,
	glmService GLMService,
	ragService RAGService,
	logger *zap.Logger,
) ChatService {
	return &chatService{
		chatRepo:   chatRepo,
		glmService: glmService,
		ragService: ragService,
		logger:     logger,
	}
}

func (s *chatService) CreateSession(ctx context.Context, userID uuid.UUID, title string) (*dto.ChatSessionResponse, error) {
	if title == "" {
		title = "Cuộc trò chuyện mới"
	}
	session := &model.ChatSession{
		UserID: userID,
		Title:  title,
	}
	if err := s.chatRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return &dto.ChatSessionResponse{
		ID:        session.ID,
		Title:     session.Title,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}, nil
}

func (s *chatService) ListSessions(ctx context.Context, userID uuid.UUID, limit int) ([]dto.ChatSessionResponse, error) {
	sessions, err := s.chatRepo.ListSessionsByUserID(ctx, userID, limit)
	if err != nil {
		return nil, err
	}

	res := make([]dto.ChatSessionResponse, 0, len(sessions))
	for _, sess := range sessions {
		res = append(res, dto.ChatSessionResponse{
			ID:        sess.ID,
			Title:     sess.Title,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
		})
	}
	return res, nil
}

func (s *chatService) GetSessionMessages(ctx context.Context, userID, sessionID uuid.UUID, limit int) (*dto.ChatSessionResponse, error) {
	sess, err := s.chatRepo.GetSessionByID(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}

	messages, err := s.chatRepo.ListMessagesBySessionID(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}

	msgResponses := make([]dto.ChatMessageResponse, 0, len(messages))
	for _, m := range messages {
		msgResponses = append(msgResponses, dto.ChatMessageResponse{
			ID:          m.ID,
			SessionID:   m.SessionID,
			Role:        string(m.Role),
			Content:     m.Content,
			Status:      string(m.Status),
			ToolCalls:   m.ToolCalls,
			ToolResults: m.ToolResults,
			CreatedAt:   m.CreatedAt,
		})
	}

	return &dto.ChatSessionResponse{
		ID:        sess.ID,
		Title:     sess.Title,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
		Messages:  msgResponses,
	}, nil
}

func (s *chatService) DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	return s.chatRepo.DeleteSession(ctx, sessionID, userID)
}

func (s *chatService) ClearSessions(ctx context.Context, userID uuid.UUID) error {
	return s.chatRepo.ClearUserSessions(ctx, userID)
}

func (s *chatService) ProcessMessageStream(ctx context.Context, userID uuid.UUID, req dto.SendMessageRequest, eventChan chan<- dto.ChatStreamEvent) error {
	defer close(eventChan)

	// 1. Resolve or create chat session
	var session *model.ChatSession
	if req.SessionID != nil && *req.SessionID != uuid.Nil {
		sess, err := s.chatRepo.GetSessionByID(ctx, *req.SessionID, userID)
		if err == nil {
			session = sess
		}
	}

	if session == nil {
		runes := []rune(strings.TrimSpace(req.Message))
		firstWords := string(runes)
		if len(runes) > 40 {
			firstWords = string(runes[:40]) + "..."
		}
		newSess := &model.ChatSession{
			UserID: userID,
			Title:  firstWords,
		}
		if err := s.chatRepo.CreateSession(ctx, newSess); err != nil {
			eventChan <- dto.ChatStreamEvent{Type: "error", ErrorMessage: "Không thể khởi tạo phiên trò chuyện"}
			return err
		}
		session = newSess
	}

	// Emit session info event
	eventChan <- dto.ChatStreamEvent{
		Type:      "session_info",
		SessionID: &session.ID,
	}

	// 2. Save user message to database
	userMsg := &model.ChatMessage{
		SessionID: session.ID,
		Role:      model.ChatRoleUser,
		Content:   req.Message,
		Status:    model.MessageStatusSuccess,
	}
	if err := s.chatRepo.CreateMessage(ctx, userMsg); err != nil {
		s.logger.Warn("failed to save user chat message", zap.Error(err))
	}

	// 3. Fallback if GLM is not configured
	if s.glmService == nil || !s.glmService.IsConfigured() {
		fallbackMsg := "⚠️ **Chưa cấu hình GLM_API_KEY.**\nVui lòng cấu hình biến môi trường `GLM_API_KEY` trong file `.env` của backend `nexo-app-api` để sử dụng mô hình `glm-4-flash` (Zhipu AI)!"
		aiMsg := &model.ChatMessage{
			SessionID: session.ID,
			Role:      model.ChatRoleModel,
			Content:   fallbackMsg,
			Status:    model.MessageStatusSuccess,
		}
		_ = s.chatRepo.CreateMessage(ctx, aiMsg)

		eventChan <- dto.ChatStreamEvent{Type: "text_delta", Delta: fallbackMsg, SessionID: &session.ID, MessageID: &aiMsg.ID, Status: "SUCCESS"}
		eventChan <- dto.ChatStreamEvent{Type: "done", SessionID: &session.ID, MessageID: &aiMsg.ID, Status: "SUCCESS"}
		return nil
	}

	// Create AI message placeholder with STREAMING status
	aiMsg := &model.ChatMessage{
		SessionID: session.ID,
		Role:      model.ChatRoleModel,
		Content:   "",
		Status:    model.MessageStatusStreaming,
	}
	if err := s.chatRepo.CreateMessage(ctx, aiMsg); err != nil {
		s.logger.Warn("failed to create ai chat message placeholder", zap.Error(err))
	}

	// Emit session info event with AI message ID
	eventChan <- dto.ChatStreamEvent{
		Type:      "session_info",
		SessionID: &session.ID,
		MessageID: &aiMsg.ID,
		Status:    "STREAMING",
	}

	// 4. RAG Step: Search matching knowledge documents from Knowledge Base
	eventChan <- dto.ChatStreamEvent{
		Type:      "tool_start",
		ToolTitle: "Đang tra cứu từ Kho tri thức Tài chính Nexo (RAG)...",
		SessionID: &session.ID,
		MessageID: &aiMsg.ID,
		Status:    "STREAMING",
	}

	knowledgeResults, err := s.ragService.SearchKnowledge(ctx, req.Message, 3)
	if err != nil {
		s.logger.Warn("error searching knowledge base", zap.Error(err))
	}

	var knowledgeContext strings.Builder
	if len(knowledgeResults) > 0 {
		var titles []string
		for idx, doc := range knowledgeResults {
			titles = append(titles, doc.Title)
			knowledgeContext.WriteString(fmt.Sprintf("\n--- TÀI LIỆU TRI THỨC [%d]: %s (Chủ đề: %s) ---\n%s\n", idx+1, doc.Title, doc.Topic, doc.Content))
		}

		eventChan <- dto.ChatStreamEvent{
			Type: "action_card",
			ActionCard: &dto.ActionCard{
				ActionType:  "KNOWLEDGE_SOURCE",
				Title:       "Kho Tri thức Tài chính Nexo RAG",
				Description: fmt.Sprintf("Trích xuất %d nguồn: %s", len(knowledgeResults), strings.Join(titles, ", ")),
				Data:        knowledgeResults,
			},
			SessionID: &session.ID,
			MessageID: &aiMsg.ID,
			Status:    "STREAMING",
		}
	} else {
		knowledgeContext.WriteString("(Không có tài liệu nào trong cơ sở tri thức khớp với câu hỏi)")
	}

	eventChan <- dto.ChatStreamEvent{
		Type:      "tool_done",
		SessionID: &session.ID,
		MessageID: &aiMsg.ID,
		Status:    "STREAMING",
	}

	// 5. Build System Prompt & Messages for GLM-4
	nowStr := time.Now().Format("2006-01-02")
	systemPrompt := fmt.Sprintf(`Bạn là Nexo AI Advisor - Trợ lý Cố vấn Tri thức Tài chính Cá nhân của Nexo.
Hôm nay là: %s.

=== DỮ LIỆU TRI THỨC ĐƯỢC CUNG CẤP (RAG CONTEXT) ===
%s
=====================================================

CÁC QUY TẮC BẮT BUỘC KHI TRẢ LỜI (GROUNDED RAG RULES):
1. CHỈ TRẢ LỜI TRONG NGỮ CẢNH: Bạn CHỈ ĐƯỢC PHÉP trả lời dựa trên những thông tin, dữ liệu thực tế có trong phần "RAG CONTEXT" ở trên.
2. TUYỆT ĐỐI KHÔNG BỊA ĐẶT (NO HALLUCINATION): Nghiêm cấm hoàn toàn việc tự suy diễn, phỏng đoán, thêm thắt hoặc bịa ra câu trả lời không có căn cứ trong tài liệu được cung cấp.
3. NẾU KHÔNG CÓ THÔNG TIN: Nếu "RAG CONTEXT" trống hoặc không chứa thông tin để trả lời câu hỏi của người dùng, bạn PHẢI TRẢ LỜI DỨT KHOÁT: "Hiện tại trong cơ sở tri thức của hệ thống không có thông tin về vấn đề này. Tôi không thể cung cấp câu trả lời khi chưa có tài liệu xác thực." Tuyệt đối không cố gắng trả lời từ kiến thức bên ngoài.
4. ĐỊNH DẠNG: Trình bày bằng tiếng Việt mạch lạc, chuyên nghiệp, sử dụng Markdown (bullet points, in đậm từ khóa quan trọng) và bám sát chính xác nội dung trong tài liệu trích xuất.`, nowStr, knowledgeContext.String())

	// 6. Load recent conversation history
	recentMessages, err := s.chatRepo.ListMessagesBySessionID(ctx, session.ID, 8)
	if err != nil {
		s.logger.Warn("failed to list recent messages", zap.Error(err))
	}

	glmMessages := make([]GLMMessage, 0, len(recentMessages)+1)
	glmMessages = append(glmMessages, GLMMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	for _, msg := range recentMessages {
		if msg.ID == aiMsg.ID {
			continue // Skip current placeholder
		}
		role := "user"
		if msg.Role == model.ChatRoleModel {
			role = "assistant"
		}
		glmMessages = append(glmMessages, GLMMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// 7. Stream generated answer tokens directly to user via GLM-4
	var fullAIResponse strings.Builder
	err = s.glmService.StreamChatCompletions(ctx, glmMessages, func(delta string) error {
		if delta != "" {
			fullAIResponse.WriteString(delta)
			eventChan <- dto.ChatStreamEvent{
				Type:      "text_delta",
				Delta:     delta,
				SessionID: &session.ID,
				MessageID: &aiMsg.ID,
				Status:    "STREAMING",
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("failed to stream answer from GLM", zap.Error(err))
		errorContent := "Có lỗi khi kết nối tới AI: " + err.Error()
		if fullAIResponse.Len() > 0 {
			errorContent = fullAIResponse.String() + "\n\n⚠️ " + errorContent
		}
		_ = s.chatRepo.UpdateMessage(ctx, aiMsg.ID, errorContent, model.MessageStatusError)

		eventChan <- dto.ChatStreamEvent{
			Type:         "error",
			ErrorMessage: err.Error(),
			SessionID:    &session.ID,
			MessageID:    &aiMsg.ID,
			Status:       "ERROR",
		}
	} else {
		// Update status to SUCCESS in DB
		_ = s.chatRepo.UpdateMessage(ctx, aiMsg.ID, fullAIResponse.String(), model.MessageStatusSuccess)

		eventChan <- dto.ChatStreamEvent{
			Type:      "done",
			SessionID: &session.ID,
			MessageID: &aiMsg.ID,
			Status:    "SUCCESS",
		}
	}

	return nil
}
