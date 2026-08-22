package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type NLPHandler struct {
	nlpService service.NLPService
	validator  *validator.Validate
	log        *zap.Logger
}

func NewNLPHandler(nlpService service.NLPService, log *zap.Logger) *NLPHandler {
	return &NLPHandler{
		nlpService: nlpService,
		validator:  validator.New(),
		log:        log,
	}
}

// ParseNLP parses natural language input for transaction fields
// @Summary Parse natural language transaction input
// @Description Parse Vietnamese text input into structured transaction fields (amount, category, type, description)
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ParseNLPRequest true "Parse NLP Request"
// @Success 200 {object} dto.ParseNLPResponse
// @Failure 400 {object} string "Bad Request"
// @Failure 500 {object} string "Internal Server Error"
// @Router /transactions/parse-nlp [post]
func (h *NLPHandler) ParseNLP(w http.ResponseWriter, r *http.Request) {
	var req dto.ParseNLPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode NLP request body", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userIDVal := r.Context().Value(constant.UserContextKey)
	var userID uuid.UUID
	if u, ok := userIDVal.(model.User); ok {
		userID = u.ID
	}

	res, err := h.nlpService.ParseTransaction(r.Context(), userID, req.Text)
	if err != nil {
		h.log.Error("failed to parse transaction input", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response.BaseResponse[dto.ParseNLPResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *res,
	})
}
