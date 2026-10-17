package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type ChatService interface {
	CreateSession(ctx context.Context, userID uuid.UUID, title string) (*dto.ChatSessionResponse, error)
	ListSessions(ctx context.Context, userID uuid.UUID, limit int) ([]dto.ChatSessionResponse, error)
	GetSessionMessages(ctx context.Context, userID, sessionID uuid.UUID, limit int) (*dto.ChatSessionResponse, error)
	DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error
	ClearSessions(ctx context.Context, userID uuid.UUID) error
	ListAvailableModels(ctx context.Context) ([]string, error)
	ProcessMessageStream(ctx context.Context, userID uuid.UUID, req dto.SendMessageRequest, eventChan chan<- dto.ChatStreamEvent) error
	// ConfirmAction executes an AI write proposal (same user only, once).
	ConfirmAction(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error)
	// CancelAction discards an AI write proposal.
	CancelAction(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error)
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
	pendingActions     PendingActionStore
	historyMu          sync.Mutex
	now                func() time.Time
}

// ChatServiceOption configures optional collaborators of the chat service.
type ChatServiceOption func(*chatService)

// WithPendingActionStore replaces the default in-memory proposal store.
func WithPendingActionStore(store PendingActionStore) ChatServiceOption {
	return func(s *chatService) { s.pendingActions = store }
}

// WithChatClock overrides the clock (tests).
func WithChatClock(now func() time.Time) ChatServiceOption {
	return func(s *chatService) { s.now = now }
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
	opts ...ChatServiceOption,
) ChatService {
	s := &chatService{
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
		now:                util.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.pendingActions == nil {
		s.pendingActions = NewInMemoryPendingActionStore(PendingActionTTL, time.Now)
	}
	if s.logger == nil {
		s.logger = zap.NewNop()
	}
	return s
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

	// Newest `limit` messages, returned oldest-first (chronological) so the UI
	// shows the latest conversation; limit <= 0 returns the whole session.
	var messages []model.ChatMessage
	if limit > 0 {
		messages, err = s.chatRepo.ListRecentMessagesBySessionID(ctx, sessionID, limit)
	} else {
		messages, err = s.chatRepo.ListMessagesBySessionID(ctx, sessionID, 0)
	}
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

const (
	// chatHistoryMaxMessages is how many stored messages of the session are
	// sent upstream as context.
	chatHistoryMaxMessages = 8
	// chatHistoryMaxMessageRunes truncates each history message.
	chatHistoryMaxMessageRunes = 4000
	// chatHistoryMaxTotalRunes caps the history sent upstream; the oldest
	// messages are dropped first.
	chatHistoryMaxTotalRunes = 16000
)

// buildUpstreamMessages builds the system prompt + capped conversation
// history (oldest → newest) sent to the model. skipID is the current AI
// placeholder message.
func buildUpstreamMessages(systemPrompt string, history []model.ChatMessage, skipID uuid.UUID) []RequestyMessage {
	if len(history) > chatHistoryMaxMessages {
		history = history[len(history)-chatHistoryMaxMessages:]
	}

	kept := make([]RequestyMessage, 0, len(history))
	total := 0
	// Walk newest → oldest so the latest turns survive the total budget.
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.ID == skipID || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		content := msg.Content
		if runes := []rune(content); len(runes) > chatHistoryMaxMessageRunes {
			content = string(runes[:chatHistoryMaxMessageRunes]) + "…"
		}
		n := utf8.RuneCountInString(content)
		if total+n > chatHistoryMaxTotalRunes && len(kept) > 0 {
			break
		}
		total += n

		role := "user"
		if msg.Role == model.ChatRoleModel {
			role = "assistant"
		}
		kept = append(kept, RequestyMessage{Role: role, Content: content})
	}

	out := make([]RequestyMessage, 0, len(kept)+1)
	out = append(out, RequestyMessage{Role: "system", Content: systemPrompt})
	for i := len(kept) - 1; i >= 0; i-- {
		out = append(out, kept[i])
	}
	return out
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
	nowStr := util.Now().Format("2006-01-02")
	systemPrompt := fmt.Sprintf(`Bạn là Nexo AI Advisor - Trợ lý Cố vấn & Quản lý Tài chính Cá nhân thông minh của ứng dụng Nexo.
Hôm nay là: %s.

Bạn ĐƯỢC TRANG BỊ các công cụ (tools) để tra cứu dữ liệu tài chính thực tế và thực hiện tác vụ cho người dùng. 
BẮT BUỘC: Khi người dùng hỏi hoặc yêu cầu bất kỳ điều gì liên quan đến dữ liệu cá nhân (số dư, giao dịch, chi tiêu, thu nhập, ngân sách, nợ, ví), bạn PHẢI GỌI TOOL tương ứng. KHÔNG ĐƯỢC tự suy đoán hay bịa ra số liệu.

Hướng dẫn gọi tool cụ thể:
1. Tổng quan tài chính (thu/chi/số dư/tiết kiệm) -> gọi "get_financial_overview".
2. Danh mục chi tiêu, cơ cấu chi tiêu -> gọi "get_spending_by_category".
3. Xem giao dịch gần đây -> gọi "list_recent_transactions"; tìm kiếm/lọc nâng cao -> gọi "search_transactions".
4. Ghi nhận/thêm chi tiêu hoặc thu nhập (ví dụ: "vừa ăn phở 50k", "thêm chi tiêu 100k", "nhận lương 25tr") -> gọi "create_transaction".
5. Tạo mới danh mục thu/chi/đầu tư -> gọi "create_category".
6. Ngân sách & hạn mức chi tiêu -> gọi "get_budget_status".
7. Các khoản nợ / cho vay -> gọi "get_debt_summary" hoặc "get_debt_detail".
8. Số dư ví, danh sách tài khoản -> gọi "list_wallets" hoặc "get_wallet_detail".
9. Danh mục kèm mức chi tháng này -> gọi "list_categories".
10. Xu hướng chi tiêu qua các tháng -> gọi "get_monthly_trend".
11. Danh mục đầu tư, tài sản -> gọi "get_investment_summary".
12. Kiến thức tài chính: NẾU trong ngữ cảnh đã có "KIẾN THỨC NỘI BỘ LIÊN QUAN" -> dùng ngay thông tin đó và KHÔNG gọi "search_financial_knowledge"; CHỈ gọi tool này khi thiếu thông tin.

Quy tắc với các tool GHI dữ liệu (create_transaction, create_category, transfer_between_wallets, create_debt, record_debt_repayment, set_budget, set_financial_target):
- Gọi tool sẽ chỉ TẠO ĐỀ XUẤT, CHƯA lưu gì. Kết quả "pending_confirmation" nghĩa là đang chờ người dùng bấm "Xác nhận" trên thẻ đề xuất.
- TUYỆT ĐỐI KHÔNG nói đã lưu/đã ghi/đã chuyển thành công. Hãy tóm tắt đề xuất và nhắc người dùng kiểm tra rồi bấm "Xác nhận" (hoặc "Huỷ").

Quy tắc trả lời sau khi nhận kết quả từ Tool:
- Trả lời bằng tiếng Việt tự nhiên, ngắn gọn, súc tích, thân thiện và chuyên nghiệp.
- Sử dụng định dạng Markdown (in đậm số tiền, bullet points, bảng).
- Định dạng tiền tệ chuẩn Việt Nam (ví dụ: 50.000 ₫, 12.500.000 ₫).
- Đưa ra nhận xét hoặc lời khuyên hữu ích dựa trên số liệu thực tế vừa tra cứu.`, nowStr)

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
	recentMessages, err := s.chatRepo.ListRecentMessagesBySessionID(ctx, session.ID, chatHistoryMaxMessages)
	if err != nil {
		s.logger.Warn("failed to list recent messages", zap.Error(err))
	}

	requestyMessages := buildUpstreamMessages(systemPrompt, recentMessages, aiMsg.ID)

	requestedModel := ""
	if req.Model != nil {
		requestedModel = *req.Model
	}

	// 6. Check for Tool Calling
	// If knowledge was already auto-injected, hide the retrieval tool for this
	// turn so the model cannot duplicate the lookup (next turn re-injects).
	tools := GetFinancialToolDefinitions()
	if len(injectedDocs) > 0 {
		tools = withoutKnowledgeSearchTool(tools)
	}
	toolChatResp, err := s.requestyService.ChatCompletionWithModel(ctx, requestedModel, requestyMessages, tools)

	var fullAIResponse strings.Builder
	// turnCards are the tool action cards of this turn, stored with the AI
	// message so a reloaded session shows them (and their confirm status).
	var turnCards []dto.ActionCard
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
			hasProposal := false

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

				var toolRes *FinancialToolResult
				var toolErr error
				if IsChatWriteTool(tc.Function.Name) {
					// Writes need the user's confirmation: validate and propose only.
					toolRes, toolErr = s.proposeWriteTool(ctx, userID, session.ID, aiMsg.ID, tc)
				} else {
					toolRes, toolErr = s.executeFinancialTool(ctx, userID, tc)
				}
				if toolErr == nil && toolRes == nil {
					toolRes = &FinancialToolResult{ToolTitle: "Lỗi thực thi công cụ", ResultJSON: toolErrorJSON("Công cụ không trả về kết quả")}
				}
				if toolErr != nil {
					s.logger.Error("tool execution error", zap.String("tool", tc.Function.Name), zap.Error(toolErr))
					errJSON, _ := json.Marshal(map[string]string{"error": toolErr.Error()})
					toolRes = &FinancialToolResult{
						ToolTitle:  "Lỗi thực thi công cụ",
						ResultJSON: string(errJSON),
					}
				}

				if toolRes.ActionCard != nil {
					turnCards = append(turnCards, *toolRes.ActionCard)
					if toolRes.ActionCard.ActionType == ActionTypePendingConfirmation {
						hasProposal = true
					}
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
			streamFilter := util.NewThinkingStreamFilter()
			err = s.requestyService.StreamChatCompletionsWithModel(ctx, requestedModel, requestyMessages, func(delta string) error {
				cleanDelta := streamFilter.Process(delta)
				if cleanDelta != "" {
					fullAIResponse.WriteString(cleanDelta)
					eventChan <- dto.ChatStreamEvent{
						Type:      "text_delta",
						Delta:     cleanDelta,
						SessionID: &session.ID,
						MessageID: &aiMsg.ID,
						Status:    "STREAMING",
					}
				}
				return nil
			})
			if flushed := streamFilter.Flush(); flushed != "" {
				fullAIResponse.WriteString(flushed)
				eventChan <- dto.ChatStreamEvent{
					Type:      "text_delta",
					Delta:     flushed,
					SessionID: &session.ID,
					MessageID: &aiMsg.ID,
					Status:    "STREAMING",
				}
			}

			// If AI model stream returned empty content after running tools, synthesize a fallback response from tool results
			if strings.TrimSpace(fullAIResponse.String()) == "" && len(fallbackSummaries) > 0 {
				fallbackPrefix := "Đã hoàn thành yêu cầu của bạn:\n\n"
				if hasProposal {
					fallbackPrefix = "Mình đã chuẩn bị đề xuất, bạn kiểm tra và bấm **Xác nhận** để lưu:\n\n"
				}
				fallbackText := fallbackPrefix + strings.Join(fallbackSummaries, "\n")
				fullAIResponse.Reset()
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
			// Model responded directly with text - strip thinking tags and stream token-by-token for silky smooth UX
			cleanContent := util.StripThinkingTags(choice.Message.Content)
			if cleanContent != "" {
				fullAIResponse.WriteString(cleanContent)
				runes := []rune(cleanContent)
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
			}
		} else {
			// Fallback to stream completions
			streamFilter := util.NewThinkingStreamFilter()
			err = s.requestyService.StreamChatCompletionsWithModel(ctx, requestedModel, requestyMessages, func(delta string) error {
				cleanDelta := streamFilter.Process(delta)
				if cleanDelta != "" {
					fullAIResponse.WriteString(cleanDelta)
					eventChan <- dto.ChatStreamEvent{
						Type:      "text_delta",
						Delta:     cleanDelta,
						SessionID: &session.ID,
						MessageID: &aiMsg.ID,
						Status:    "STREAMING",
					}
				}
				return nil
			})
			if flushed := streamFilter.Flush(); flushed != "" {
				fullAIResponse.WriteString(flushed)
				eventChan <- dto.ChatStreamEvent{
					Type:      "text_delta",
					Delta:     flushed,
					SessionID: &session.ID,
					MessageID: &aiMsg.ID,
					Status:    "STREAMING",
				}
			}
		}
	} else {
		// Fallback to direct stream completions
		streamFilter := util.NewThinkingStreamFilter()
		err = s.requestyService.StreamChatCompletionsWithModel(ctx, requestedModel, requestyMessages, func(delta string) error {
			cleanDelta := streamFilter.Process(delta)
			if cleanDelta != "" {
				fullAIResponse.WriteString(cleanDelta)
				eventChan <- dto.ChatStreamEvent{
					Type:      "text_delta",
					Delta:     cleanDelta,
					SessionID: &session.ID,
					MessageID: &aiMsg.ID,
					Status:    "STREAMING",
				}
			}
			return nil
		})
		if flushed := streamFilter.Flush(); flushed != "" {
			fullAIResponse.WriteString(flushed)
			eventChan <- dto.ChatStreamEvent{
				Type:      "text_delta",
				Delta:     flushed,
				SessionID: &session.ID,
				MessageID: &aiMsg.ID,
				Status:    "STREAMING",
			}
		}
	}

	// Persist the final message even when the client has disconnected (ctx
	// cancelled); otherwise it would stay STREAMING forever.
	persistCtx, cancelPersist := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelPersist()

	if err != nil {
		s.logger.Error("failed to stream answer from AI", zap.Error(err))
		errorContent := "Có lỗi khi kết nối tới AI: " + err.Error()
		if fullAIResponse.Len() > 0 {
			errorContent = fullAIResponse.String() + "\n\n⚠️ " + errorContent
		}
		_ = s.chatRepo.UpdateMessage(persistCtx, aiMsg.ID, errorContent, model.MessageStatusError)
		s.persistActionCards(persistCtx, userID, aiMsg.ID, turnCards)

		eventChan <- dto.ChatStreamEvent{
			Type:         "error",
			ErrorMessage: err.Error(),
			SessionID:    &session.ID,
			MessageID:    &aiMsg.ID,
			Status:       "ERROR",
		}
	} else {
		// Update status to SUCCESS in DB
		_ = s.chatRepo.UpdateMessage(persistCtx, aiMsg.ID, fullAIResponse.String(), model.MessageStatusSuccess)
		s.persistActionCards(persistCtx, userID, aiMsg.ID, turnCards)

		eventChan <- dto.ChatStreamEvent{
			Type:      "done",
			SessionID: &session.ID,
			MessageID: &aiMsg.ID,
			Status:    "SUCCESS",
		}
	}

	return nil
}

func (s *chatService) ListAvailableModels(ctx context.Context) ([]string, error) {
	return s.requestyService.ListModels(ctx)
}
