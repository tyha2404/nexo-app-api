package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

const (
	// notConfiguredMessage is the whole answer when the AI gateway is not configured.
	notConfiguredMessage = "⚠️ **Chưa cấu hình API Key.**\nVui lòng cấu hình các biến môi trường `NINE_ROUTER_*` (`NINE_ROUTER_URL`, `NINE_ROUTER_API_KEY`) trong file `.env` của backend `nexo-app-api` để sử dụng trợ lý AI!"
	// streamClientErrorMessage replaces upstream errors, which can carry the
	// gateway URL or raw response body (those stay in the log).
	streamClientErrorMessage = "Có lỗi khi kết nối tới AI, vui lòng thử lại sau"
	// directAnswerChunkRunes / directAnswerChunkDelay replay a non-streamed
	// answer token-by-token for a smooth UI.
	directAnswerChunkRunes = 4
	directAnswerChunkDelay = 15 * time.Millisecond
)

// chatTurn is the AI answer being produced for one user message: the events
// it emits all carry its session and message ids.
type chatTurn struct {
	ch        chan<- dto.ChatStreamEvent
	sessionID uuid.UUID
	messageID uuid.UUID
	answer    strings.Builder
	// cards are the tool action cards of this turn, stored with the AI
	// message so a reloaded session shows them (and their confirm status).
	cards []dto.ActionCard
}

// send emits ev for this turn's session and message with the given status.
func (t *chatTurn) send(ev dto.ChatStreamEvent, status model.ChatMessageStatus) {
	ev.SessionID, ev.MessageID, ev.Status = &t.sessionID, &t.messageID, string(status)
	t.ch <- ev
}

// emit sends a STREAMING event.
func (t *chatTurn) emit(ev dto.ChatStreamEvent) { t.send(ev, model.MessageStatusStreaming) }

// emitText appends delta to the answer and streams it.
func (t *chatTurn) emitText(delta string) {
	t.answer.WriteString(delta)
	t.emit(dto.ChatStreamEvent{Type: "text_delta", Delta: delta})
}

func (s *chatService) ProcessMessageStream(ctx context.Context, userID uuid.UUID, req dto.SendMessageRequest, eventChan chan<- dto.ChatStreamEvent) error {
	defer close(eventChan)

	session, err := s.resolveSession(ctx, userID, req)
	if err != nil {
		eventChan <- dto.ChatStreamEvent{Type: "error", ErrorMessage: "Không thể khởi tạo phiên trò chuyện"}
		return err
	}
	eventChan <- dto.ChatStreamEvent{Type: "session_info", SessionID: &session.ID}

	userMsg := &model.ChatMessage{SessionID: session.ID, Role: model.ChatRoleUser, Content: req.Message, Status: model.MessageStatusSuccess}
	if err := s.chatRepo.CreateMessage(ctx, userMsg); err != nil {
		s.logger.Warn("failed to save user chat message", zap.Error(err))
	}

	if s.nineRouterService == nil || !s.nineRouterService.IsConfigured() {
		s.answerNotConfigured(ctx, session.ID, eventChan)
		return nil
	}

	// AI message placeholder, completed when the turn ends.
	aiMsg := &model.ChatMessage{SessionID: session.ID, Role: model.ChatRoleModel, Status: model.MessageStatusStreaming}
	if err := s.chatRepo.CreateMessage(ctx, aiMsg); err != nil {
		s.logger.Warn("failed to create ai chat message placeholder", zap.Error(err))
	}
	turn := &chatTurn{ch: eventChan, sessionID: session.ID, messageID: aiMsg.ID}
	turn.emit(dto.ChatStreamEvent{Type: "session_info"})

	systemPrompt := chatSystemPrompt(util.Now().Format(time.DateOnly))
	// Ground general financial-advice questions with internal knowledge (RAG)
	// without a tool round-trip, and show which docs were used.
	injectedDocs := s.autoInjectKnowledge(ctx, req.Message, &systemPrompt, session.ID)
	if len(injectedDocs) > 0 {
		turn.emit(dto.ChatStreamEvent{Type: "action_card", ActionCard: knowledgeAutoInjectCard(req.Message, injectedDocs)})
	}

	recentMessages, err := s.chatRepo.ListRecentMessagesBySessionID(ctx, session.ID, chatHistoryMaxMessages)
	if err != nil {
		s.logger.Warn("failed to list recent messages", zap.Error(err))
	}
	upstream := buildUpstreamMessages(systemPrompt, recentMessages, aiMsg.ID)

	requestedModel := ""
	if req.Model != nil {
		requestedModel = *req.Model
	}
	// Knowledge already injected: hide the retrieval tool for this turn so the
	// model cannot duplicate the lookup (the next turn re-injects).
	tools := GetFinancialToolDefinitions()
	if len(injectedDocs) > 0 {
		tools = withoutKnowledgeSearchTool(tools)
	}

	answerErr := s.generateAnswer(ctx, userID, turn, requestedModel, upstream, tools)
	s.finishTurn(ctx, userID, turn, answerErr)
	return nil
}

// resolveSession returns the user's session req.SessionID, or a new session
// titled with the first 40 runes of the message (also when the id is unknown
// or belongs to another user).
func (s *chatService) resolveSession(ctx context.Context, userID uuid.UUID, req dto.SendMessageRequest) (*model.ChatSession, error) {
	if req.SessionID != nil && *req.SessionID != uuid.Nil {
		if sess, err := s.chatRepo.GetSessionByID(ctx, userID, *req.SessionID); err == nil {
			return sess, nil
		}
	}
	runes := []rune(strings.TrimSpace(req.Message))
	title := string(runes)
	if len(runes) > 40 {
		title = string(runes[:40]) + "..."
	}
	sess := &model.ChatSession{UserID: userID, Title: title}
	if err := s.chatRepo.CreateSession(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// answerNotConfigured stores and sends the "AI not configured" answer.
func (s *chatService) answerNotConfigured(ctx context.Context, sessionID uuid.UUID, ch chan<- dto.ChatStreamEvent) {
	aiMsg := &model.ChatMessage{SessionID: sessionID, Role: model.ChatRoleModel, Content: notConfiguredMessage, Status: model.MessageStatusSuccess}
	_ = s.chatRepo.CreateMessage(ctx, aiMsg)
	turn := &chatTurn{ch: ch, sessionID: sessionID, messageID: aiMsg.ID}
	turn.send(dto.ChatStreamEvent{Type: "text_delta", Delta: notConfiguredMessage}, model.MessageStatusSuccess)
	turn.send(dto.ChatStreamEvent{Type: "done"}, model.MessageStatusSuccess)
}

// generateAnswer asks the model (with tools) and streams the answer of the
// turn. A failed or empty tool-call request falls back to plain streaming.
func (s *chatService) generateAnswer(ctx context.Context, userID uuid.UUID, turn *chatTurn, requestedModel string, upstream []NineRouterMessage, tools []ToolDefinition) error {
	resp, err := s.nineRouterService.ChatCompletionWithModel(ctx, requestedModel, upstream, tools)
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		return s.streamAnswer(ctx, turn, requestedModel, upstream)
	}
	msg := resp.Choices[0].Message
	switch {
	case len(msg.ToolCalls) > 0:
		if len(msg.ToolCalls) > maxToolCallsPerTurn {
			// Drop the extra calls from the assistant message too: every
			// tool_call it carries must be answered by a tool message.
			s.logger.Warn("truncating tool calls", zap.Int("requested", len(msg.ToolCalls)))
			msg.ToolCalls = msg.ToolCalls[:maxToolCallsPerTurn]
		}
		var outcome toolRoundOutcome
		upstream = append(upstream, NineRouterMessage{Role: "assistant", Content: msg.Content, ToolCalls: msg.ToolCalls})
		upstream, outcome = s.runToolCalls(ctx, userID, turn, upstream, msg.ToolCalls)
		err := s.streamAnswer(ctx, turn, requestedModel, upstream)
		// The model answered nothing after the tools: summarize their cards.
		if strings.TrimSpace(turn.answer.String()) == "" && len(outcome.summaries) > 0 {
			prefix := "Đã hoàn thành yêu cầu của bạn:\n\n"
			if outcome.hasProposal {
				prefix = "Mình đã chuẩn bị đề xuất, bạn kiểm tra và bấm **Xác nhận** để lưu:\n\n"
			}
			turn.answer.Reset()
			turn.emitText(prefix + strings.Join(outcome.summaries, "\n"))
		}
		return err
	case msg.Content != "":
		// Direct answer: strip thinking tags and replay it in small chunks.
		runes := []rune(util.StripThinkingTags(msg.Content))
		for i := 0; i < len(runes); i += directAnswerChunkRunes {
			end := min(i+directAnswerChunkRunes, len(runes))
			turn.emitText(string(runes[i:end]))
			time.Sleep(directAnswerChunkDelay)
		}
		return nil
	default:
		return s.streamAnswer(ctx, turn, requestedModel, upstream)
	}
}

// streamAnswer streams the model's answer without its thinking blocks.
func (s *chatService) streamAnswer(ctx context.Context, turn *chatTurn, requestedModel string, upstream []NineRouterMessage) error {
	filter := util.NewThinkingStreamFilter()
	err := s.nineRouterService.StreamChatCompletionsWithModel(ctx, requestedModel, upstream, func(delta string) error {
		if clean := filter.Process(delta); clean != "" {
			turn.emitText(clean)
		}
		return nil
	})
	if flushed := filter.Flush(); flushed != "" {
		turn.emitText(flushed)
	}
	return err
}

// toolRoundOutcome summarizes the tool calls of a turn for the fallback answer.
type toolRoundOutcome struct {
	summaries   []string
	hasProposal bool
}

// runToolCalls runs (read tools) or proposes (write tools) each call, emits
// its events and appends one tool message per call to upstream.
func (s *chatService) runToolCalls(ctx context.Context, userID uuid.UUID, turn *chatTurn, upstream []NineRouterMessage, calls []ToolCall) ([]NineRouterMessage, toolRoundOutcome) {
	var outcome toolRoundOutcome
	for _, tc := range calls {
		turn.emit(dto.ChatStreamEvent{Type: "tool_start", ToolName: tc.Function.Name, ToolTitle: fmt.Sprintf("Đang xử lý %s...", tc.Function.Name)})

		res := s.runToolCall(ctx, userID, turn, tc)
		if card := res.ActionCard; card != nil {
			turn.cards = append(turn.cards, *card)
			if card.ActionType == ActionTypePendingConfirmation {
				outcome.hasProposal = true
			}
			if card.Description != "" {
				outcome.summaries = append(outcome.summaries, fmt.Sprintf("**%s**: %s", card.Title, card.Description))
			}
			turn.emit(dto.ChatStreamEvent{Type: "action_card", ActionCard: card})
		}
		turn.emit(dto.ChatStreamEvent{Type: "tool_done"})

		upstream = append(upstream, NineRouterMessage{Role: "tool", ToolCallID: tc.ID, Content: res.ResultJSON})
	}
	return upstream, outcome
}

// runToolCall returns the result of one tool call; errors become a tool
// error result (system errors are hidden from the model and the user).
func (s *chatService) runToolCall(ctx context.Context, userID uuid.UUID, turn *chatTurn, tc ToolCall) *FinancialToolResult {
	var (
		res *FinancialToolResult
		err error
	)
	if IsChatWriteTool(tc.Function.Name) {
		// Writes need the user's confirmation: validate and propose only.
		res, err = s.proposeWriteTool(ctx, userID, turn.sessionID, turn.messageID, tc)
	} else {
		res, err = s.executeFinancialTool(ctx, userID, tc)
	}
	if err != nil {
		s.logger.Error("tool execution error", zap.String("tool", tc.Function.Name), zap.Error(err))
		return &FinancialToolResult{ToolTitle: "Lỗi thực thi công cụ", ResultJSON: toolErrorJSON(toolErrorMessage(err))}
	}
	if res == nil {
		return &FinancialToolResult{ToolTitle: "Lỗi thực thi công cụ", ResultJSON: toolErrorJSON("Công cụ không trả về kết quả")}
	}
	return res
}

// finishTurn stores the final AI message (even when the client disconnected,
// otherwise it would stay STREAMING forever) and emits done or error.
func (s *chatService) finishTurn(ctx context.Context, userID uuid.UUID, turn *chatTurn, answerErr error) {
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if answerErr != nil {
		s.logger.Error("failed to stream answer from AI", zap.Error(answerErr))
		content := streamClientErrorMessage
		if turn.answer.Len() > 0 {
			content = turn.answer.String() + "\n\n⚠️ " + content
		}
		_ = s.chatRepo.UpdateMessage(persistCtx, userID, turn.messageID, content, model.MessageStatusError)
		s.persistActionCards(persistCtx, userID, turn.messageID, turn.cards)
		turn.send(dto.ChatStreamEvent{Type: "error", ErrorMessage: streamClientErrorMessage}, model.MessageStatusError)
		return
	}
	_ = s.chatRepo.UpdateMessage(persistCtx, userID, turn.messageID, turn.answer.String(), model.MessageStatusSuccess)
	s.persistActionCards(persistCtx, userID, turn.messageID, turn.cards)
	turn.send(dto.ChatStreamEvent{Type: "done"}, model.MessageStatusSuccess)
}

// chatSystemPrompt is the system prompt of the AI advisor for the given day.
// Its write-tool list is generated from the tool registry.
func chatSystemPrompt(today string) string {
	return fmt.Sprintf(`Bạn là Nexo AI Advisor - Trợ lý Cố vấn & Quản lý Tài chính Cá nhân thông minh của ứng dụng Nexo.
Hôm nay là: %s.

Bạn ĐƯỢC TRANG BỊ các công cụ (tools) để tra cứu dữ liệu tài chính thực tế và thực hiện tác vụ cho người dùng.`+
		// The trailing space is part of the prompt (kept byte-identical).
		" \n"+`BẮT BUỘC: Khi người dùng hỏi hoặc yêu cầu bất kỳ điều gì liên quan đến dữ liệu cá nhân (số dư, giao dịch, chi tiêu, thu nhập, ngân sách, nợ, ví), bạn PHẢI GỌI TOOL tương ứng. KHÔNG ĐƯỢC tự suy đoán hay bịa ra số liệu.

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

Quy tắc với các tool GHI dữ liệu (%s):
- Gọi tool sẽ chỉ TẠO ĐỀ XUẤT, CHƯA lưu gì. Kết quả "pending_confirmation" nghĩa là đang chờ người dùng bấm "Xác nhận" trên thẻ đề xuất.
- TUYỆT ĐỐI KHÔNG nói đã lưu/đã ghi/đã chuyển thành công. Hãy tóm tắt đề xuất và nhắc người dùng kiểm tra rồi bấm "Xác nhận" (hoặc "Huỷ").

Quy tắc trả lời sau khi nhận kết quả từ Tool:
- Trả lời bằng tiếng Việt tự nhiên, ngắn gọn, súc tích, thân thiện và chuyên nghiệp.
- Sử dụng định dạng Markdown (in đậm số tiền, bullet points, bảng).
- Định dạng tiền tệ chuẩn Việt Nam (ví dụ: 50.000 ₫, 12.500.000 ₫).
- Đưa ra nhận xét hoặc lời khuyên hữu ích dựa trên số liệu thực tế vừa tra cứu.`, today, writeToolListForPrompt())
}
