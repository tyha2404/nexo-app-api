package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
)

// PendingActionTTL is how long an AI write proposal can be confirmed.
const PendingActionTTL = 15 * time.Minute

// pendingActionRetention keeps finished/expired actions around after expiry
// so a late confirm gets 409/410 instead of 404.
const pendingActionRetention = time.Hour

var (
	// ErrPendingActionNotFound: unknown id, or the action belongs to another user.
	ErrPendingActionNotFound = fmt.Errorf("pending action %w", constant.ErrNotFound)
	// ErrPendingActionExpired: the proposal was not confirmed within PendingActionTTL.
	ErrPendingActionExpired = errors.New("pending action expired")
	// ErrPendingActionHandled: already confirmed, cancelled or being executed.
	ErrPendingActionHandled = errors.New("pending action already handled")
	// ErrPendingActionFailed: executing the confirmed action was rejected
	// (validation/business rule); the proposal stays pending.
	ErrPendingActionFailed = errors.New("pending action failed")
)

// PendingActionStatus is the lifecycle state of an AI write proposal.
type PendingActionStatus string

const (
	PendingActionPending   PendingActionStatus = "PENDING"
	PendingActionExecuting PendingActionStatus = "EXECUTING"
	PendingActionConfirmed PendingActionStatus = "CONFIRMED"
	PendingActionCancelled PendingActionStatus = "CANCELLED"
	PendingActionExpired   PendingActionStatus = "EXPIRED"
)

// PendingAction is a validated, not yet executed AI write tool call.
type PendingAction struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	SessionID uuid.UUID
	MessageID uuid.UUID
	ToolName  string
	// Args are the normalized tool arguments executed on confirm.
	Args       map[string]interface{}
	Status     PendingActionStatus
	ResultCard *dto.ActionCard
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// PendingActionStore keeps proposals until they are confirmed, cancelled or expire.
type PendingActionStore interface {
	Save(action PendingAction)
	// Get returns a copy of the user's action (status reflects expiry).
	Get(userID, id uuid.UUID) (PendingAction, error)
	// Begin moves a PENDING action to EXECUTING (single winner for concurrent confirms).
	Begin(userID, id uuid.UUID) (PendingAction, error)
	// Complete finishes an EXECUTING action.
	Complete(id uuid.UUID, resultCard *dto.ActionCard)
	// Revert puts an EXECUTING action back to PENDING after a failed execution.
	Revert(id uuid.UUID)
	// Cancel moves a PENDING action to CANCELLED.
	Cancel(userID, id uuid.UUID) (PendingAction, error)
}

type inMemoryPendingActionStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	actions map[uuid.UUID]*PendingAction
}

// NewInMemoryPendingActionStore returns a process-local store. Proposals do not
// survive a restart; confirming one afterwards returns ErrPendingActionNotFound.
func NewInMemoryPendingActionStore(ttl time.Duration, now func() time.Time) PendingActionStore {
	if ttl <= 0 {
		ttl = PendingActionTTL
	}
	if now == nil {
		now = time.Now
	}
	return &inMemoryPendingActionStore{ttl: ttl, now: now, actions: map[uuid.UUID]*PendingAction{}}
}

func (s *inMemoryPendingActionStore) prune(now time.Time) {
	for id, a := range s.actions {
		if now.After(a.ExpiresAt.Add(pendingActionRetention)) {
			delete(s.actions, id)
		}
	}
}

func (s *inMemoryPendingActionStore) Save(action PendingAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.prune(now)
	if action.CreatedAt.IsZero() {
		action.CreatedAt = now
	}
	if action.ExpiresAt.IsZero() {
		action.ExpiresAt = action.CreatedAt.Add(s.ttl)
	}
	if action.Status == "" {
		action.Status = PendingActionPending
	}
	s.actions[action.ID] = &action
}

// lookup returns the user's action with expiry applied; caller holds mu.
func (s *inMemoryPendingActionStore) lookup(userID, id uuid.UUID) (*PendingAction, error) {
	a, ok := s.actions[id]
	if !ok || a.UserID != userID {
		return nil, ErrPendingActionNotFound
	}
	if a.Status == PendingActionPending && s.now().After(a.ExpiresAt) {
		a.Status = PendingActionExpired
	}
	return a, nil
}

func (s *inMemoryPendingActionStore) Get(userID, id uuid.UUID) (PendingAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.lookup(userID, id)
	if err != nil {
		return PendingAction{}, err
	}
	return *a, nil
}

func checkPending(a *PendingAction) error {
	switch a.Status {
	case PendingActionPending:
		return nil
	case PendingActionExpired:
		return ErrPendingActionExpired
	default:
		return ErrPendingActionHandled
	}
}

func (s *inMemoryPendingActionStore) Begin(userID, id uuid.UUID) (PendingAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.lookup(userID, id)
	if err != nil {
		return PendingAction{}, err
	}
	if err := checkPending(a); err != nil {
		return *a, err
	}
	a.Status = PendingActionExecuting
	return *a, nil
}

func (s *inMemoryPendingActionStore) Complete(id uuid.UUID, resultCard *dto.ActionCard) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.actions[id]; ok && a.Status == PendingActionExecuting {
		a.Status = PendingActionConfirmed
		a.ResultCard = resultCard
	}
}

func (s *inMemoryPendingActionStore) Revert(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.actions[id]; ok && a.Status == PendingActionExecuting {
		a.Status = PendingActionPending
	}
}

func (s *inMemoryPendingActionStore) Cancel(userID, id uuid.UUID) (PendingAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.lookup(userID, id)
	if err != nil {
		return PendingAction{}, err
	}
	if err := checkPending(a); err != nil {
		return *a, err
	}
	a.Status = PendingActionCancelled
	return *a, nil
}
