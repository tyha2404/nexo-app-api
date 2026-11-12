package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type stubChatService struct {
	service.ChatService
	listSessionsFn func(ctx context.Context, userID uuid.UUID, limit int) ([]dto.ChatSessionResponse, error)
	getMessagesFn  func(ctx context.Context, userID, sessionID uuid.UUID, limit int) (*dto.ChatSessionResponse, error)
	deleteFn       func(ctx context.Context, userID, sessionID uuid.UUID) error
	clearFn        func(ctx context.Context, userID uuid.UUID) error
	modelsFn       func(ctx context.Context) ([]string, error)
	confirmFn      func(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error)
	cancelFn       func(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error)
}

func (s *stubChatService) ListSessions(ctx context.Context, userID uuid.UUID, limit int) ([]dto.ChatSessionResponse, error) {
	return s.listSessionsFn(ctx, userID, limit)
}
func (s *stubChatService) GetSessionMessages(ctx context.Context, userID, sessionID uuid.UUID, limit int) (*dto.ChatSessionResponse, error) {
	return s.getMessagesFn(ctx, userID, sessionID, limit)
}
func (s *stubChatService) DeleteSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	return s.deleteFn(ctx, userID, sessionID)
}
func (s *stubChatService) ClearSessions(ctx context.Context, userID uuid.UUID) error {
	return s.clearFn(ctx, userID)
}
func (s *stubChatService) ListAvailableModels(ctx context.Context) ([]string, error) {
	return s.modelsFn(ctx)
}
func (s *stubChatService) ConfirmAction(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error) {
	return s.confirmFn(ctx, userID, actionID)
}
func (s *stubChatService) CancelAction(ctx context.Context, userID, actionID uuid.UUID) (*dto.ChatActionResult, error) {
	return s.cancelFn(ctx, userID, actionID)
}

func TestChatHandler_ListSessions(t *testing.T) {
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		var gotUser uuid.UUID
		var gotLimit int
		svc := &stubChatService{listSessionsFn: func(ctx context.Context, uid uuid.UUID, limit int) ([]dto.ChatSessionResponse, error) {
			gotUser, gotLimit = uid, limit
			return []dto.ChatSessionResponse{{ID: uuid.New(), Title: "Hi"}}, nil
		}}
		h := NewChatHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.ListSessions(rr, categoryTestReq(http.MethodGet, "/chat/sessions", "", userID, ""))

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, userID, gotUser)
		assert.Equal(t, 30, gotLimit)
		body := decodeJSON(t, rr)
		assert.Equal(t, true, body["success"])
		require.Len(t, body["data"], 1)
		assert.Equal(t, "Hi", body["data"].([]interface{})[0].(map[string]interface{})["title"])
	})

	t.Run("unauthenticated", func(t *testing.T) {
		h := NewChatHandler(&stubChatService{}, zap.NewNop())
		rr := httptest.NewRecorder()
		h.ListSessions(rr, categoryTestReq(http.MethodGet, "/chat/sessions", "", uuid.Nil, ""))
		categoryAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
	})

	t.Run("service error", func(t *testing.T) {
		svc := &stubChatService{listSessionsFn: func(ctx context.Context, uid uuid.UUID, limit int) ([]dto.ChatSessionResponse, error) {
			return nil, errors.New("db")
		}}
		h := NewChatHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.ListSessions(rr, categoryTestReq(http.MethodGet, "/chat/sessions", "", userID, ""))
		categoryAssertError(t, rr, http.StatusInternalServerError, "Failed to list chat sessions")
	})
}

func TestChatHandler_GetSessionMessages(t *testing.T) {
	userID := uuid.New()
	sessID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, sessID.String(), nil, http.StatusOK, ""},
		{"unauthenticated", uuid.Nil, sessID.String(), nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid uuid", userID, "x", nil, http.StatusBadRequest, "Invalid session ID"},
		{"not found", userID, sessID.String(), fmt.Errorf("session %w", constant.ErrNotFound), http.StatusNotFound, "Session not found"},
		{"service error", userID, sessID.String(), errors.New("db"), http.StatusInternalServerError, "Failed to get session messages"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotSess uuid.UUID
			var gotLimit int
			svc := &stubChatService{getMessagesFn: func(ctx context.Context, uid, sid uuid.UUID, limit int) (*dto.ChatSessionResponse, error) {
				gotUser, gotSess, gotLimit = uid, sid, limit
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &dto.ChatSessionResponse{ID: sid, Title: "T"}, nil
			}}
			h := NewChatHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.GetSessionMessages(rr, categoryTestReq(http.MethodGet, "/chat/sessions/x", "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, sessID, gotSess)
			assert.Equal(t, 50, gotLimit)
			assert.Equal(t, sessID.String(), decodeJSON(t, rr)["data"].(map[string]interface{})["id"])
		})
	}
}

func TestChatHandler_DeleteSession(t *testing.T) {
	userID := uuid.New()
	sessID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, sessID.String(), nil, http.StatusOK, ""},
		{"unauthenticated", uuid.Nil, sessID.String(), nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid uuid", userID, "x", nil, http.StatusBadRequest, "Invalid session ID"},
		{"not found", userID, sessID.String(), constant.ErrNotFound, http.StatusNotFound, "Session not found"},
		{"service error", userID, sessID.String(), errors.New("db"), http.StatusInternalServerError, "Failed to delete session"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotSess uuid.UUID
			svc := &stubChatService{deleteFn: func(ctx context.Context, uid, sid uuid.UUID) error {
				gotUser, gotSess = uid, sid
				return tt.svcErr
			}}
			h := NewChatHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.DeleteSession(rr, categoryTestReq(http.MethodDelete, "/chat/sessions/x", "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, sessID, gotSess)
			assert.JSONEq(t, `{"status":200,"success":true,"message":"Phiên trò chuyện đã được xóa thành công","data":null}`, rr.Body.String())
		})
	}
}

func TestChatHandler_ClearSessions(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, nil, http.StatusOK, ""},
		{"unauthenticated", uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"service error", userID, errors.New("db"), http.StatusInternalServerError, "Failed to clear chat sessions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			svc := &stubChatService{clearFn: func(ctx context.Context, uid uuid.UUID) error {
				gotUser = uid
				return tt.svcErr
			}}
			h := NewChatHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.ClearSessions(rr, categoryTestReq(http.MethodPost, "/chat/clear", "", tt.user, ""))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, true, decodeJSON(t, rr)["success"])
		})
	}
}

func TestChatHandler_ListModels(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		err    error
		want   []interface{}
	}{
		{"from gateway", []string{"gpt-x", "claude-y"}, nil, []interface{}{"gpt-x", "claude-y"}},
		{"gateway down falls back to auto", nil, errors.New("timeout"), []interface{}{"auto"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubChatService{modelsFn: func(ctx context.Context) ([]string, error) {
				return tt.models, tt.err
			}}
			h := NewChatHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.ListModels(rr, httptest.NewRequest(http.MethodGet, "/chat/models", nil))

			require.Equal(t, http.StatusOK, rr.Code)
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, tt.want, body["data"].(map[string]interface{})["models"])
		})
	}
}

func TestChatHandler_HandleActionError(t *testing.T) {
	h := NewChatHandler(&stubChatService{}, zap.NewNop())
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"handled", service.ErrPendingActionHandled, http.StatusConflict, "Đề xuất này đã được xử lý trước đó"},
		{"expired", service.ErrPendingActionExpired, http.StatusGone, "Đề xuất đã hết hạn, vui lòng yêu cầu lại trợ lý AI"},
		{"not found", service.ErrPendingActionNotFound, http.StatusNotFound, "Không tìm thấy đề xuất (có thể đã hết hạn)"},
		{"plain not found is not mapped to 404", constant.ErrNotFound, http.StatusInternalServerError, "Không thể xử lý đề xuất"},
		{"failed carries the reason", fmt.Errorf("%w: %s", service.ErrPendingActionFailed, "Ví không đủ số dư"), http.StatusUnprocessableEntity, "Ví không đủ số dư"},
		{"other", errors.New("boom"), http.StatusInternalServerError, "Không thể xử lý đề xuất"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.handleActionError(rr, tt.err, "op")
			categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
		})
	}
}

func TestChatHandler_ConfirmAndCancelAction(t *testing.T) {
	userID := uuid.New()
	actionID := uuid.New()

	type call func(h *ChatHandler) http.HandlerFunc
	endpoints := []struct {
		name       string
		call       call
		wantStatus string
	}{
		{"confirm", func(h *ChatHandler) http.HandlerFunc { return h.ConfirmAction }, "CONFIRMED"},
		{"cancel", func(h *ChatHandler) http.HandlerFunc { return h.CancelAction }, "CANCELLED"},
	}
	cases := []struct {
		name       string
		user       uuid.UUID
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, actionID.String(), nil, http.StatusOK, ""},
		{"unauthenticated", uuid.Nil, actionID.String(), nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid uuid", userID, "x", nil, http.StatusBadRequest, "Invalid action ID"},
		{"another user's action", userID, actionID.String(), service.ErrPendingActionNotFound, http.StatusNotFound, ""},
		{"already handled", userID, actionID.String(), service.ErrPendingActionHandled, http.StatusConflict, ""},
		{"expired", userID, actionID.String(), service.ErrPendingActionExpired, http.StatusGone, ""},
		{"internal", userID, actionID.String(), errors.New("db"), http.StatusInternalServerError, ""},
	}
	for _, ep := range endpoints {
		for _, tc := range cases {
			t.Run(ep.name+" "+tc.name, func(t *testing.T) {
				var gotUser, gotAction uuid.UUID
				fn := func(ctx context.Context, uid, aid uuid.UUID) (*dto.ChatActionResult, error) {
					gotUser, gotAction = uid, aid
					if tc.svcErr != nil {
						return nil, tc.svcErr
					}
					return &dto.ChatActionResult{ActionID: aid, Status: ep.wantStatus, ToolName: "create_transaction"}, nil
				}
				svc := &stubChatService{confirmFn: fn, cancelFn: fn}
				h := NewChatHandler(svc, zap.NewNop())
				rr := httptest.NewRecorder()
				ep.call(h)(rr, categoryTestReq(http.MethodPost, "/chat/actions/x", "", tc.user, tc.id))

				if tc.wantStatus != http.StatusOK {
					categoryAssertError(t, rr, tc.wantStatus, tc.wantMsg)
					return
				}
				require.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, userID, gotUser, "action must be scoped to the caller")
				assert.Equal(t, actionID, gotAction)
				data := decodeJSON(t, rr)["data"].(map[string]interface{})
				assert.Equal(t, ep.wantStatus, data["status"])
				assert.Equal(t, actionID.String(), data["actionId"])
			})
		}
	}
}
