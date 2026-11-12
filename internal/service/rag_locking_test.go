package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"go.uber.org/zap"
)

// memKnowledgeRepo is an in-memory knowledge table.
type memKnowledgeRepo struct {
	repository.KnowledgeRepository
	mu   sync.Mutex
	docs []model.FinancialKnowledge
}

func (r *memKnowledgeRepo) ListAll(context.Context) ([]model.FinancialKnowledge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]model.FinancialKnowledge(nil), r.docs...), nil
}

func (r *memKnowledgeRepo) ReplaceByTopic(_ context.Context, topic string, docs []*model.FinancialKnowledge) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.docs[:0]
	for _, d := range r.docs {
		if d.Topic != topic {
			kept = append(kept, d)
		}
	}
	for _, d := range docs {
		d.ID = uuid.New()
		kept = append(kept, *d)
	}
	r.docs = kept
	return nil
}

// slowEmbedder blocks embedding calls for texts containing "CHẬM" until
// release is closed; other texts embed at once (3 dimensions).
type slowEmbedder struct {
	NineRouterService
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (e *slowEmbedder) IsConfigured() bool { return true }
func (e *slowEmbedder) EmbedText(_ context.Context, text string) ([]float32, error) {
	if strings.Contains(text, "CHẬM") {
		e.once.Do(func() { close(e.started) })
		<-e.release
	}
	return []float32{1, 0, 0}, nil
}

func TestRAG_SearchIsNotBlockedByEmbeddingWriter(t *testing.T) {
	repo := &memKnowledgeRepo{docs: []model.FinancialKnowledge{{ID: uuid.New(), Topic: "budget", Title: "50/30/20", Content: "chia thu nhập", Embedding: "[1,0,0]"}}}
	emb := &slowEmbedder{started: make(chan struct{}), release: make(chan struct{})}
	s := &ragService{knowledgeRepo: repo, nineRouterService: emb, logger: zap.NewNop(), localEmbCache: map[uuid.UUID]cachedDocEmbedding{}}

	added := make(chan error, 1)
	go func() { added <- s.AddKnowledge(context.Background(), "slow", "CHẬM", "nội dung mới") }()
	<-emb.started // the writer is now waiting on the embedding gateway

	searched := make(chan []KnowledgeSearchResult, 1)
	go func() {
		res, err := s.SearchKnowledge(context.Background(), "thu nhập", 3)
		assert.NoError(t, err)
		searched <- res
	}()
	select {
	case res := <-searched:
		require.NotEmpty(t, res)
		assert.Equal(t, "budget", res[0].Topic)
	case <-time.After(2 * time.Second):
		t.Fatal("search blocked while a knowledge writer was embedding")
	}

	close(emb.release)
	require.NoError(t, <-added)
	docs, _ := repo.ListAll(context.Background())
	assert.Len(t, docs, 2, "the slow topic was written after its embedding")
}

func TestRAG_ReplacingATopicEvictsItsCachedEmbeddings(t *testing.T) {
	budgetID, debtID := uuid.New(), uuid.New()
	// 2-dimension stored embeddings vs 3-dimension query: the local fallback
	// vectors get cached per doc id.
	repo := &memKnowledgeRepo{docs: []model.FinancialKnowledge{
		{ID: budgetID, Topic: "budget", Title: "Ngân sách", Content: "quy tắc chia lương", Embedding: "[1,0]"},
		{ID: debtID, Topic: "debt", Title: "Nợ", Content: "trả nợ snowball", Embedding: "[0,1]"},
	}}
	emb := &slowEmbedder{started: make(chan struct{}), release: make(chan struct{})}
	s := &ragService{knowledgeRepo: repo, nineRouterService: emb, logger: zap.NewNop(), localEmbCache: map[uuid.UUID]cachedDocEmbedding{}}

	_, err := s.SearchKnowledge(context.Background(), "chia lương", 3)
	require.NoError(t, err)
	require.Contains(t, s.localEmbCache, budgetID)
	require.Contains(t, s.localEmbCache, debtID)

	require.NoError(t, s.AddKnowledge(context.Background(), "budget", "Ngân sách", "nội dung mới"))
	assert.NotContains(t, s.localEmbCache, budgetID, "the replaced row's cache entry is evicted")
	assert.Contains(t, s.localEmbCache, debtID, "other topics keep their cache")
}
