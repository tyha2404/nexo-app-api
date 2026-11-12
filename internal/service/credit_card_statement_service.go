package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

var (
	ErrStatementNotFound       = repository.ErrStatementNotFound
	ErrInvalidDate             = constant.NewKindError(constant.ErrInvalidInput, "định dạng ngày không hợp lệ (cần YYYY-MM-DD)")
	ErrStatementAlreadyPaid    = repository.ErrStatementAlreadyPaid
	ErrStatementInvalidPayment = repository.ErrStatementInvalidPayment
)

type CreditCardStatementService interface {
	CreateStatement(ctx context.Context, userID uuid.UUID, req dto.CreateStatementRequest) (*dto.StatementResponse, error)
	GetStatementByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*dto.StatementResponse, error)
	ListStatements(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) (*dto.StatementListResponse, error)
	UpdateStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.UpdateStatementRequest) (*dto.StatementResponse, error)
	PayStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.PayStatementRequest) (*dto.StatementResponse, error)
	DeleteStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type creditCardStatementService struct {
	repo       repository.CreditCardStatementRepository
	walletRepo repository.WalletRepository
}

func NewCreditCardStatementService(
	repo repository.CreditCardStatementRepository,
	walletRepo repository.WalletRepository,
) CreditCardStatementService {
	return &creditCardStatementService{
		repo:       repo,
		walletRepo: walletRepo,
	}
}

// Default minimum payment when none is given: defaultMinimumPaymentRate of
// the statement balance, but at least minimumPaymentFloor (VND) when the
// balance itself reaches that floor.
const (
	defaultMinimumPaymentRate = 0.05
	minimumPaymentFloor       = 50000
)

func parseDateString(str string) (time.Time, error) {
	t, err := util.ParseDate(str)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse(time.RFC3339, str)
	if err == nil {
		return t, nil
	}
	return time.Time{}, ErrInvalidDate
}

func (s *creditCardStatementService) CreateStatement(ctx context.Context, userID uuid.UUID, req dto.CreateStatementRequest) (*dto.StatementResponse, error) {
	wallet, err := s.walletRepo.GetByIDForUser(ctx, userID, req.WalletID)
	if err != nil {
		return nil, err
	}

	stmtDate, err := parseDateString(req.StatementDate)
	if err != nil {
		return nil, err
	}

	dueDate, err := parseDateString(req.DueDate)
	if err != nil {
		return nil, err
	}

	minPay := req.MinimumPayment
	if minPay <= 0 && req.StatementBalance > 0 {
		minPay = req.StatementBalance * defaultMinimumPaymentRate
		if minPay < minimumPaymentFloor && req.StatementBalance >= minimumPaymentFloor {
			minPay = minimumPaymentFloor
		}
	}

	status := model.StatementStatusFor(req.StatementBalance, req.PaidAmount)

	note := ""
	if req.Note != nil {
		note = *req.Note
	}

	stmt := &model.CreditCardStatement{
		ID:               uuid.New(),
		UserID:           userID,
		WalletID:         req.WalletID,
		StatementMonth:   req.StatementMonth,
		StatementYear:    req.StatementYear,
		StatementDate:    stmtDate,
		DueDate:          dueDate,
		StatementBalance: req.StatementBalance,
		MinimumPayment:   minPay,
		PreviousBalance:  req.PreviousBalance,
		PaidAmount:       req.PaidAmount,
		Status:           status,
		Note:             note,
	}

	if err := s.repo.Create(ctx, stmt); err != nil {
		return nil, err
	}

	// Also sync latest active statement to wallet (snapshot columns only; never
	// a full-row Save, which could overwrite a concurrent balance change).
	if err := s.walletRepo.UpdateStatementSnapshot(ctx, userID, wallet.ID, req.StatementBalance, minPay, req.PreviousBalance); err != nil {
		return nil, err
	}
	wallet.StatementBalance = &req.StatementBalance
	wallet.MinimumPayment = &minPay
	wallet.PreviousBalance = &req.PreviousBalance

	stmt.Wallet = wallet
	res := dto.ToStatementResponse(stmt)
	return &res, nil
}

func (s *creditCardStatementService) GetStatementByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*dto.StatementResponse, error) {
	stmt, err := s.repo.GetByIDForUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	res := dto.ToStatementResponse(stmt)
	return &res, nil
}

func (s *creditCardStatementService) ListStatements(ctx context.Context, userID uuid.UUID, walletID *uuid.UUID, year *int, month *int) (*dto.StatementListResponse, error) {
	items, err := s.repo.List(ctx, userID, walletID, year, month)
	if err != nil {
		return nil, err
	}

	resItems := make([]dto.StatementResponse, 0, len(items))
	var totalBal, totalPaid, totalRemaining float64

	for _, item := range items {
		resp := dto.ToStatementResponse(&item)
		totalBal += resp.StatementBalance
		totalPaid += resp.PaidAmount
		totalRemaining += resp.RemainingAmount
		resItems = append(resItems, resp)
	}

	return &dto.StatementListResponse{
		Items:          resItems,
		Total:          int64(len(resItems)),
		TotalBalance:   totalBal,
		TotalPaid:      totalPaid,
		TotalRemaining: totalRemaining,
	}, nil
}

// UpdateStatement edits a statement. Dates are validated up front (invalid →
// ErrInvalidDate, like CreateStatement). The changes are applied on the row
// locked by the repository, so fields the request leaves out (notably
// paid_amount, which PayStatement changes concurrently) keep their committed
// value and the status is recomputed from it.
func (s *creditCardStatementService) UpdateStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.UpdateStatementRequest) (*dto.StatementResponse, error) {
	var stmtDate, dueDate *time.Time
	if req.StatementDate != nil {
		d, err := parseDateString(*req.StatementDate)
		if err != nil {
			return nil, err
		}
		stmtDate = &d
	}
	if req.DueDate != nil {
		d, err := parseDateString(*req.DueDate)
		if err != nil {
			return nil, err
		}
		dueDate = &d
	}

	stmt, err := s.repo.Update(ctx, userID, id, func(stmt *model.CreditCardStatement) error {
		if req.StatementMonth != nil {
			stmt.StatementMonth = *req.StatementMonth
		}
		if req.StatementYear != nil {
			stmt.StatementYear = *req.StatementYear
		}
		if stmtDate != nil {
			stmt.StatementDate = *stmtDate
		}
		if dueDate != nil {
			stmt.DueDate = *dueDate
		}
		if req.StatementBalance != nil {
			stmt.StatementBalance = *req.StatementBalance
		}
		if req.MinimumPayment != nil {
			stmt.MinimumPayment = *req.MinimumPayment
		}
		if req.PreviousBalance != nil {
			stmt.PreviousBalance = *req.PreviousBalance
		}
		if req.PaidAmount != nil {
			stmt.PaidAmount = *req.PaidAmount
		}
		if req.Note != nil {
			stmt.Note = *req.Note
		}
		// The status always follows the amounts (req.Status is not trusted).
		stmt.Status = model.StatementStatusFor(stmt.StatementBalance, stmt.PaidAmount)
		return nil
	})
	if errors.Is(err, constant.ErrNotFound) {
		return nil, ErrStatementNotFound
	}
	if err != nil {
		return nil, err
	}

	res := dto.ToStatementResponse(stmt)
	return &res, nil
}

// PayStatement records a payment. All checks (statement lock, not already PAID,
// no overpayment, source-wallet ownership) and all balance changes happen in a
// single DB transaction in the repository; any failure rolls everything back.
func (s *creditCardStatementService) PayStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID, req dto.PayStatementRequest) (*dto.StatementResponse, error) {
	stmt, err := s.repo.PayStatement(ctx, userID, id, req.Amount, req.SourceWalletID)
	if err != nil {
		return nil, err
	}
	res := dto.ToStatementResponse(stmt)
	return &res, nil
}

func (s *creditCardStatementService) DeleteStatement(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}
