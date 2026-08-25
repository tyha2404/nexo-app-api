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
	chatRepo           repository.ChatRepository
	requestyService    RequestyService
	ragService         RAGService
	transactionService TransactionService
	reportService      ReportService
	budgetService      BudgetService
	debtService        DebtService
	walletService      WalletService
	targetService      TargetService
	categoryRepo       repository.CategoryRepo
	transactionRepo    repository.TransactionRepository
	walletRepo         repository.WalletRepository
	logger             *zap.Logger
}

func NewChatService(
	chatRepo repository.ChatRepository,
	requestyService RequestyService,
	ragService RAGService,
	transactionService TransactionService,
	reportService ReportService,
	budgetService BudgetService,
	debtService DebtService,
	walletService WalletService,
	targetService TargetService,
	categoryRepo repository.CategoryRepo,
	transactionRepo repository.TransactionRepository,
	walletRepo repository.WalletRepository,
	logger *zap.Logger,
) ChatService {
	return &chatService{
		chatRepo:           chatRepo,
		requestyService:    requestyService,
		ragService:         ragService,
		transactionService: transactionService,
		reportService:      reportService,
		budgetService:      budgetService,
		debtService:        debtService,
		walletService:      walletService,
		targetService:      targetService,
		categoryRepo:       categoryRepo,
		transactionRepo:    transactionRepo,
		walletRepo:         walletRepo,
		logger:             logger,
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

const (
	// ragAutoInjectTopK is the number of knowledge docs auto-injected per turn.
	ragAutoInjectTopK = 2
	// ragAutoInjectMinScore filters out weak semantic matches; fallback-flagged
	// results are always excluded regardless of this threshold.
	ragAutoInjectMinScore = 0.30
)

// autoInjectKnowledge grounds the system prompt with internal knowledge that
// semantically matches the user's message, avoiding a tool round-trip for
// general financial-advice questions. Failures never block the chat flow.
// Returns the injected docs so the caller can render a citation card.
func (s *chatService) autoInjectKnowledge(ctx context.Context, userMessage string, systemPrompt *string, sessionID uuid.UUID) []KnowledgeSearchResult {
	if s.ragService == nil || strings.TrimSpace(userMessage) == "" {
		return nil
	}

	results, err := s.ragService.SearchKnowledge(ctx, userMessage, ragAutoInjectTopK)
	if err != nil {
		s.logger.Warn("rag auto-inject search failed", zap.Error(err))
		return nil
	}

	confident := make([]KnowledgeSearchResult, 0, len(results))
	for _, r := range results {
		if !r.Fallback && r.Score >= ragAutoInjectMinScore {
			confident = append(confident, r)
		}
	}
	if len(confident) == 0 {
		return nil
	}

	var kb strings.Builder
	kb.WriteString("\n\nKIẾN THỨC NỘI BỘ LIÊN QUAN (tra cứu tự động theo câu hỏi, ưu tiên dùng nếu phù hợp):\n")
	for _, r := range confident {
		fmt.Fprintf(&kb, "- [%s] %s\n%s\n", r.Topic, r.Title, r.Content)
	}

	*systemPrompt += kb.String()
	s.logger.Info("rag auto-injected knowledge into system prompt",
		zap.String("session_id", sessionID.String()),
		zap.Int("docs", len(confident)),
	)
	return confident
}

// knowledgeAutoInjectCard builds the citation card shown for silently
// auto-injected knowledge, mirroring the tool's KNOWLEDGE_SOURCE card.
func knowledgeAutoInjectCard(query string, docs []KnowledgeSearchResult) *dto.ActionCard {
	return &dto.ActionCard{
		ActionType:  "KNOWLEDGE_SOURCE",
		Title:       "Nguồn tri thức nội bộ",
		Description: fmt.Sprintf("Tự động tra cứu: %d tài liệu liên quan tới \"%s\"", len(docs), query),
		Data: map[string]interface{}{
			"query":   query,
			"results": docs,
			"source":  "auto_inject",
		},
	}
}

// withoutKnowledgeSearchTool removes the retrieval tool from the definitions
// for a turn where knowledge was already auto-injected, making the
// "don't re-lookup" rule structural instead of prompt-dependent.
func withoutKnowledgeSearchTool(defs []ToolDefinition) []ToolDefinition {
	filtered := make([]ToolDefinition, 0, len(defs))
	for _, d := range defs {
		if d.Function.Name == "search_financial_knowledge" {
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered
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

	// 3. Fallback if AI is not configured
	if s.requestyService == nil || !s.requestyService.IsConfigured() {
		fallbackMsg := "⚠️ **Chưa cấu hình API Key.**\nVui lòng cấu hình biến môi trường `REQUESTY_API_KEY` trong file `.env` của backend `nexo-app-api` để sử dụng trợ lý AI!"
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

	// 4. Build System Prompt & Messages directly for Model
	nowStr := time.Now().Format("2006-01-02")
	systemPrompt := fmt.Sprintf(`Bạn là Nexo AI Advisor - Trợ lý Cố vấn & Quản lý Tài chính Cá nhân thông minh của ứng dụng Nexo.
Hôm nay là: %s.

Bạn có các công cụ tài chính mạnh mẽ của hệ thống Nexo để tra cứu dữ liệu thực tế và thực hiện tác vụ cho người dùng:
1. Khi người dùng hỏi về tình hình tài chính tổng quan, thu/chi/tiết kiệm -> gọi tool "get_financial_overview".
2. Khi người dùng hỏi về danh mục chi tiêu, cơ cấu chi tiêu -> gọi tool "get_spending_by_category".
3. Khi người dùng muốn xem lịch sử giao dịch đơn giản (mới nhất) -> gọi tool "list_recent_transactions"; nếu cần tìm kiếm/lọc phức tạp theo danh mục, ví, số tiền, từ khóa hoặc sắp xếp -> gọi tool "search_transactions".
4. Khi người dùng yêu cầu ghi nhận/thêm chi tiêu hoặc thu nhập (ví dụ: "vừa ăn phở 50k", "thêm chi tiêu 100k tiền cafe", "nhận lương 25tr") -> hãy chủ động gọi tool "create_transaction" (công cụ sẽ tự động tạo danh mục mới tương ứng theo ngữ cảnh nếu danh mục đó chưa từng có trong hệ thống).
5. Khi người dùng yêu cầu tạo mới hoặc thêm một danh mục thu/chi/đầu tư cụ thể (ví dụ: "tạo danh mục Tiền điện", "thêm danh mục Học tập loại chi tiêu", "tạo danh mục Youtube loại thu nhập") -> gọi tool "create_category".
6. Khi người dùng hỏi về ngân sách, hạn mức chi tiêu -> gọi tool "get_budget_status".
7. Khi người dùng hỏi về các khoản nợ hoặc cho vay -> gọi tool "get_debt_summary"; khi cần chi tiết một khoản nợ cụ thể kèm lịch sử thanh toán -> gọi tool "get_debt_detail".
8. Khi người dùng hỏi về số dư các ví, tài khoản -> gọi tool "list_wallets"; khi cần xem chi tiết một ví cụ thể và giao dịch trên ví đó -> gọi tool "get_wallet_detail".
9. Khi người dùng cần liệt kê danh mục thu/chi (kèm mức chi tháng này) -> gọi tool "list_categories".
10. Khi người dùng hỏi về xu hướng nhiều tháng, trung bình chi tiêu/thu nhập qua các tháng -> gọi tool "get_monthly_trend".
11. Khi người dùng hỏi về đầu tư, tài sản đang nắm giữ, lãi lỗ -> gọi tool "get_investment_summary".
12. Lời khuyên về nguyên tắc quản lý tiền, chiến lược tiết kiệm/đầu tư dựa trên kho tri thức nội bộ: NẾU trong ngữ cảnh đã có phần "KIẾN THỨC NỘI BỘ LIÊN QUAN" và nó bao phủ được câu hỏi -> trả lời NGAY từ phần đó và KHÔNG gọi "search_financial_knowledge"; CHỈ gọi tool này khi phần đó bị thiếu hoặc không liên quan tới câu hỏi.

Quy tắc trả lời:
- Luôn chủ động gọi công cụ thích hợp khi người dùng yêu cầu thao tác hoặc hỏi dữ liệu tài chính cá nhân.
- Sau khi có kết quả từ công cụ, trả lời người dùng bằng tiếng Việt tự nhiên, ngắn gọn, súc tích và chuyên nghiệp, sử dụng định dạng Markdown (bullet points, in đậm số tiền, bảng nếu cần).
- Luôn định dạng tiền tệ rõ ràng theo chuẩn Việt Nam (ví dụ: 50.000 ₫, 12.500.000 ₫).
- Đưa ra lời khuyên thực tế, hữu ích giúp người dùng quản lý tài chính hiệu quả hơn.
- Nếu hệ thống đã cung cấp sẵn phần "KIẾN THỨC NỘI BỘ LIÊN QUAN" và nó trả lời được câu hỏi, dùng ngay phần đó và không tra cứu lại để tránh trùng lặp.`, nowStr)

	// 4b. Auto-inject relevant internal knowledge (RAG) into the system prompt
	// so general financial-advice questions are grounded without a tool round-trip.
	injectedDocs := s.autoInjectKnowledge(ctx, req.Message, &systemPrompt, session.ID)
	if len(injectedDocs) > 0 {
		// Citation card so the user sees which internal docs ground the answer.
		eventChan <- dto.ChatStreamEvent{
			Type:       "action_card",
			ActionCard: knowledgeAutoInjectCard(req.Message, injectedDocs),
			SessionID:  &session.ID,
			MessageID:  &aiMsg.ID,
			Status:     "STREAMING",
		}
	}

	// 5. Load recent conversation history
	recentMessages, err := s.chatRepo.ListMessagesBySessionID(ctx, session.ID, 8)
	if err != nil {
		s.logger.Warn("failed to list recent messages", zap.Error(err))
	}

	requestyMessages := make([]RequestyMessage, 0, len(recentMessages)+1)
	requestyMessages = append(requestyMessages, RequestyMessage{
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
		requestyMessages = append(requestyMessages, RequestyMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// 6. Check for Tool Calling
	// If knowledge was already auto-injected, hide the retrieval tool for this
	// turn so the model cannot duplicate the lookup (next turn re-injects).
	tools := GetFinancialToolDefinitions()
	if len(injectedDocs) > 0 {
		tools = withoutKnowledgeSearchTool(tools)
	}
	toolChatResp, err := s.requestyService.ChatCompletion(ctx, requestyMessages, tools)

	var fullAIResponse strings.Builder
	if err == nil && toolChatResp != nil && len(toolChatResp.Choices) > 0 {
		choice := toolChatResp.Choices[0]
		if len(choice.Message.ToolCalls) > 0 {
			// Append assistant tool_calls message
			requestyMessages = append(requestyMessages, RequestyMessage{
				Role:      "assistant",
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			})

			// Collect tool results summary for fallback text if model returns empty response
			var fallbackSummaries []string

			// Execute each tool call
			for _, tc := range choice.Message.ToolCalls {
				eventChan <- dto.ChatStreamEvent{
					Type:      "tool_start",
					ToolName:  tc.Function.Name,
					ToolTitle: fmt.Sprintf("Đang xử lý %s...", tc.Function.Name),
					SessionID: &session.ID,
					MessageID: &aiMsg.ID,
					Status:    "STREAMING",
				}

				toolRes, toolErr := s.executeFinancialTool(ctx, userID, tc)
				if toolErr != nil {
					s.logger.Error("tool execution error", zap.String("tool", tc.Function.Name), zap.Error(toolErr))
					toolRes = &FinancialToolResult{
						ToolTitle:  "Lỗi thực thi công cụ",
						ResultJSON: fmt.Sprintf(`{"error": "%s"}`, toolErr.Error()),
					}
				}

				if toolRes.ActionCard != nil {
					if toolRes.ActionCard.Description != "" {
						fallbackSummaries = append(fallbackSummaries, fmt.Sprintf("**%s**: %s", toolRes.ActionCard.Title, toolRes.ActionCard.Description))
					}
					eventChan <- dto.ChatStreamEvent{
						Type:       "action_card",
						ActionCard: toolRes.ActionCard,
						SessionID:  &session.ID,
						MessageID:  &aiMsg.ID,
						Status:     "STREAMING",
					}
				}

				eventChan <- dto.ChatStreamEvent{
					Type:      "tool_done",
					SessionID: &session.ID,
					MessageID: &aiMsg.ID,
					Status:    "STREAMING",
				}

				requestyMessages = append(requestyMessages, RequestyMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    toolRes.ResultJSON,
				})
			}

			// Stream final answer from model after tool outputs
			err = s.requestyService.StreamChatCompletions(ctx, requestyMessages, func(delta string) error {
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

			// If AI model stream returned empty content after running tools, synthesize a fallback response from tool results
			if fullAIResponse.Len() == 0 && len(fallbackSummaries) > 0 {
				fallbackText := "Đã hoàn thành yêu cầu của bạn:\n\n" + strings.Join(fallbackSummaries, "\n")
				fullAIResponse.WriteString(fallbackText)
				eventChan <- dto.ChatStreamEvent{
					Type:      "text_delta",
					Delta:     fallbackText,
					SessionID: &session.ID,
					MessageID: &aiMsg.ID,
					Status:    "STREAMING",
				}
			}
		} else if choice.Message.Content != "" {
			// Model responded directly with text - stream it token-by-token for silky smooth UX
			fullAIResponse.WriteString(choice.Message.Content)
			runes := []rune(choice.Message.Content)
			chunkSize := 4
			for i := 0; i < len(runes); i += chunkSize {
				end := i + chunkSize
				if end > len(runes) {
					end = len(runes)
				}
				delta := string(runes[i:end])
				eventChan <- dto.ChatStreamEvent{
					Type:      "text_delta",
					Delta:     delta,
					SessionID: &session.ID,
					MessageID: &aiMsg.ID,
					Status:    "STREAMING",
				}
				time.Sleep(15 * time.Millisecond)
			}
		} else {
			// Fallback to stream completions
			err = s.requestyService.StreamChatCompletions(ctx, requestyMessages, func(delta string) error {
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
		}
	} else {
		// Fallback to direct stream completions
		err = s.requestyService.StreamChatCompletions(ctx, requestyMessages, func(delta string) error {
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
	}

	if err != nil {
		s.logger.Error("failed to stream answer from AI", zap.Error(err))
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
