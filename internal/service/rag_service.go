package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/tyha2404/nexo-app-api/internal/data"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type KnowledgeSearchResult struct {
	Topic   string  `json:"topic"`
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float32 `json:"score"`
}

type RAGService interface {
	SeedDefaultKnowledge(ctx context.Context) error
	SearchKnowledge(ctx context.Context, query string, limit int) ([]KnowledgeSearchResult, error)
	AddKnowledge(ctx context.Context, topic, title, content string) error
}

type ragService struct {
	knowledgeRepo   repository.KnowledgeRepository
	requestyService RequestyService
	logger          *zap.Logger
	mu              sync.Mutex
}

func NewRAGService(knowledgeRepo repository.KnowledgeRepository, requestyService RequestyService, logger *zap.Logger) RAGService {
	s := &ragService{
		knowledgeRepo:   knowledgeRepo,
		requestyService: requestyService,
		logger:          logger,
	}

	// Seed knowledge asynchronously in background
	go func() {
		ctx := context.Background()
		if err := s.SeedDefaultKnowledge(ctx); err != nil {
			logger.Warn("failed to auto-seed default financial knowledge", zap.Error(err))
		}
	}()

	return s
}

func (s *ragService) generateEmbedding(ctx context.Context, text string) []float32 {
	if s.requestyService != nil && s.requestyService.IsConfigured() {
		emb, err := s.requestyService.EmbedText(ctx, text)
		if err == nil && len(emb) > 0 {
			return emb
		}
		s.logger.Warn("Requesty embed failed or unavailable, falling back to local semantic vectorizer", zap.Error(err))
	}
	return util.GenerateLocalEmbedding(text, 256)
}

func (s *ragService) SeedDefaultKnowledge(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fileDocs, err := data.LoadEmbeddedKnowledgeDocs()
	if err != nil {
		s.logger.Warn("failed to load embedded knowledge docs", zap.Error(err))
		return err
	}

	allDocs, err := s.knowledgeRepo.ListAll(ctx)
	if err != nil {
		return err
	}

	// Group existing docs by topic
	existingByTopic := make(map[string][]model.FinancialKnowledge)
	for _, d := range allDocs {
		existingByTopic[d.Topic] = append(existingByTopic[d.Topic], d)
	}

	for _, doc := range fileDocs {
		existingChunks, found := existingByTopic[doc.Topic]
		if !found || len(existingChunks) == 0 {
			// New doc added in data/knowledge -> chunk, embed, and insert
			s.logger.Info("seeding new knowledge doc with chunking", zap.String("topic", doc.Topic), zap.String("title", doc.Title))
			if err := s.addKnowledgeLocked(ctx, doc.Topic, doc.Title, doc.Content); err != nil {
				s.logger.Warn("failed to seed knowledge doc", zap.String("topic", doc.Topic), zap.Error(err))
			}
			continue
		}

		// Check if any chunk has empty/invalid embedding
		hasEmptyEmb := false
		for _, chunk := range existingChunks {
			var emb []float32
			if err := json.Unmarshal([]byte(chunk.Embedding), &emb); err != nil || len(emb) == 0 {
				hasEmptyEmb = true
				break
			}
		}

		// Check if chunk count differs from expected chunks
		expectedChunks := util.ChunkText(doc.Content, util.DefaultChunkOptions)
		hasChunkMismatch := len(existingChunks) != len(expectedChunks)

		if hasEmptyEmb || hasChunkMismatch {
			s.logger.Info("re-chunking and syncing knowledge doc", zap.String("topic", doc.Topic), zap.String("title", doc.Title))
			if err := s.addKnowledgeLocked(ctx, doc.Topic, doc.Title, doc.Content); err != nil {
				s.logger.Warn("failed to update chunked knowledge doc", zap.String("topic", doc.Topic), zap.Error(err))
			}
		}
	}

	return nil
}

func (s *ragService) AddKnowledge(ctx context.Context, topic, title, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addKnowledgeLocked(ctx, topic, title, content)
}

func (s *ragService) addKnowledgeLocked(ctx context.Context, topic, title, content string) error {
	chunks := util.ChunkText(content, util.DefaultChunkOptions)
	if len(chunks) == 0 {
		return nil
	}

	// Delete old chunks for this topic to prevent orphaned duplicates
	if err := s.knowledgeRepo.DeleteByTopic(ctx, topic); err != nil {
		s.logger.Warn("failed to delete old chunks before re-inserting", zap.String("topic", topic), zap.Error(err))
	}

	for _, chunk := range chunks {
		// Context-enhanced chunk text for embedding
		fullChunkText := title + "\n" + chunk.Content
		embedding := s.generateEmbedding(ctx, fullChunkText)
		embBytes, _ := json.Marshal(embedding)

		chunkTitle := title
		if chunk.Total > 1 {
			chunkTitle = fmt.Sprintf("%s (Phần %d/%d)", title, chunk.Index+1, chunk.Total)
		}

		doc := &model.FinancialKnowledge{
			Topic:     topic,
			Title:     chunkTitle,
			Content:   chunk.Content,
			Embedding: string(embBytes),
		}

		if err := s.knowledgeRepo.Create(ctx, doc); err != nil {
			return err
		}
	}

	return nil
}

func (s *ragService) SearchKnowledge(ctx context.Context, query string, limit int) ([]KnowledgeSearchResult, error) {
	allDocs, err := s.knowledgeRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	if len(allDocs) == 0 {
		return nil, nil
	}

	if limit <= 0 {
		limit = 3
	}

	queryEmb := s.generateEmbedding(ctx, query)
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	var results []KnowledgeSearchResult
	for _, doc := range allDocs {
		var docEmb []float32
		var score float32

		if err := json.Unmarshal([]byte(doc.Embedding), &docEmb); err == nil && len(docEmb) > 0 {
			if len(queryEmb) == len(docEmb) {
				score = ComputeCosineSimilarity(queryEmb, docEmb)
			} else {
				// Dimension fallback
				localQ := util.GenerateLocalEmbedding(query, 256)
				localD := util.GenerateLocalEmbedding(doc.Title+"\n"+doc.Content, 256)
				score = ComputeCosineSimilarity(localQ, localD)
			}
		}

		// Keyword relevance bonus
		docText := strings.ToLower(doc.Title + " " + doc.Content)
		for _, w := range queryWords {
			if len(w) > 2 && strings.Contains(docText, w) {
				score += 0.05
			}
		}

		if score > 0.05 {
			results = append(results, KnowledgeSearchResult{
				Topic:   doc.Topic,
				Title:   doc.Title,
				Content: doc.Content,
				Score:   score,
			})
		}
	}

	// Sort descending by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Deduplicate by topic so LLM receives diverse information
	var distinctResults []KnowledgeSearchResult
	seenTopic := make(map[string]bool)
	for _, res := range results {
		if !seenTopic[res.Topic] {
			seenTopic[res.Topic] = true
			distinctResults = append(distinctResults, res)
			if len(distinctResults) >= limit {
				break
			}
		}
	}

	// Fallback to top general rules if no high match
	if len(distinctResults) == 0 && len(allDocs) > 0 {
		for i := 0; i < len(allDocs) && len(distinctResults) < limit; i++ {
			if !seenTopic[allDocs[i].Topic] {
				seenTopic[allDocs[i].Topic] = true
				distinctResults = append(distinctResults, KnowledgeSearchResult{
					Topic:   allDocs[i].Topic,
					Title:   allDocs[i].Title,
					Content: allDocs[i].Content,
					Score:   0.5,
				})
			}
		}
	}

	return distinctResults, nil
}
