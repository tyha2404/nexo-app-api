package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type MockKnowledgeRepo struct {
	mock.Mock
}

func (m *MockKnowledgeRepo) Create(ctx context.Context, knowledge *model.FinancialKnowledge) error {
	args := m.Called(ctx, knowledge)
	return args.Error(0)
}

func (m *MockKnowledgeRepo) Update(ctx context.Context, knowledge *model.FinancialKnowledge) error {
	args := m.Called(ctx, knowledge)
	return args.Error(0)
}

func (m *MockKnowledgeRepo) DeleteByTopic(ctx context.Context, topic string) error {
	args := m.Called(ctx, topic)
	return args.Error(0)
}

func (m *MockKnowledgeRepo) ReplaceByTopic(ctx context.Context, topic string, docs []*model.FinancialKnowledge) error {
	args := m.Called(ctx, topic, docs)
	return args.Error(0)
}

func (m *MockKnowledgeRepo) ListAll(ctx context.Context) ([]model.FinancialKnowledge, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.FinancialKnowledge), args.Error(1)
}

func (m *MockKnowledgeRepo) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockKnowledgeRepo) GetByTopic(ctx context.Context, topic string) ([]model.FinancialKnowledge, error) {
	args := m.Called(ctx, topic)
	return args.Get(0).([]model.FinancialKnowledge), args.Error(1)
}

func (m *MockKnowledgeRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.FinancialKnowledge, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.FinancialKnowledge), args.Error(1)
}

type MockRequestyService struct {
	mock.Mock
}

func (m *MockRequestyService) IsConfigured() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockRequestyService) ChatCompletion(ctx context.Context, messages []service.RequestyMessage, tools []service.ToolDefinition) (*service.RequestyChatResponse, error) {
	args := m.Called(ctx, messages, tools)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.RequestyChatResponse), args.Error(1)
}

func (m *MockRequestyService) StreamChatCompletions(ctx context.Context, messages []service.RequestyMessage, onChunk func(delta string) error) error {
	args := m.Called(ctx, messages, onChunk)
	return args.Error(0)
}

func (m *MockRequestyService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

func TestComputeCosineSimilarity(t *testing.T) {
	vecA := []float32{1.0, 0.0, 0.0}
	vecB := []float32{1.0, 0.0, 0.0}
	assert.InDelta(t, 1.0, service.ComputeCosineSimilarity(vecA, vecB), 0.001)

	vecC := []float32{0.0, 1.0, 0.0}
	assert.InDelta(t, 0.0, service.ComputeCosineSimilarity(vecA, vecC), 0.001)

	vecD := []float32{-1.0, 0.0, 0.0}
	assert.InDelta(t, -1.0, service.ComputeCosineSimilarity(vecA, vecD), 0.001)
}

func TestRAGService_SearchKnowledge(t *testing.T) {
	mockRepo := new(MockKnowledgeRepo)
	mockAI := new(MockRequestyService)
	logger := zap.NewNop()

	docList := []model.FinancialKnowledge{
		{
			ID:        uuid.New(),
			Topic:     "50_30_20_rule",
			Title:     "Quy tắc 50/30/20",
			Content:   "50% nhu cầu thiết yếu, 30% mong muốn, 20% tiết kiệm.",
			Embedding: "[0.1, 0.2, 0.3]",
		},
	}

	mockRepo.On("ListAll", mock.Anything).Return(docList, nil)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("DeleteByTopic", mock.Anything, mock.Anything).Return(nil).Maybe()
	// Background seeding goroutine may upsert any embedded knowledge topic
	mockRepo.On("ReplaceByTopic", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockAI.On("IsConfigured").Return(true)
	mockAI.On("EmbedText", mock.Anything, mock.Anything).Return([]float32{0.1, 0.2, 0.3}, nil)

	ragSvc := service.NewRAGService(mockRepo, mockAI, logger)

	results, err := ragSvc.SearchKnowledge(context.Background(), "tư vấn 50/30/20", 2)
	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, "50_30_20_rule", results[0].Topic)
	assert.True(t, results[0].Score > 0.5)
}

func TestRAGService_BackfillEmptyEmbedding(t *testing.T) {
	mockRepo := new(MockKnowledgeRepo)
	mockAI := new(MockRequestyService)
	logger := zap.NewNop()

	docWithEmptyEmb := []model.FinancialKnowledge{
		{
			ID:        uuid.New(),
			Topic:     "50_30_20_rule",
			Title:     "Quy tắc 50/30/20",
			Content:   "50% nhu cầu thiết yếu, 30% mong muốn, 20% tiết kiệm.",
			Embedding: "[]",
		},
	}

	mockRepo.On("ListAll", mock.Anything).Return(docWithEmptyEmb, nil)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("DeleteByTopic", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("ReplaceByTopic", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockAI.On("IsConfigured").Return(false)

	ragSvc := service.NewRAGService(mockRepo, mockAI, logger)
	err := ragSvc.SeedDefaultKnowledge(context.Background())
	assert.NoError(t, err)
}

func TestRAGService_SearchKnowledge_FallbackFlaggedNotFaked(t *testing.T) {
	mockRepo := new(MockKnowledgeRepo)
	mockAI := new(MockRequestyService)
	logger := zap.NewNop()

	// Doc with an empty embedding can never match semantically, and the query
	// shares no keywords -> the search must fall back WITHOUT faking a score.
	docList := []model.FinancialKnowledge{
		{
			ID:        uuid.New(),
			Topic:     "emergency_fund",
			Title:     "Quỹ dự khẩn",
			Content:   "Cần tích lũy quỹ dự phòng tối thiểu sáu tháng chi tiêu.",
			Embedding: "[]",
		},
	}

	mockRepo.On("ListAll", mock.Anything).Return(docList, nil)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("DeleteByTopic", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("ReplaceByTopic", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockAI.On("IsConfigured").Return(true)
	mockAI.On("EmbedText", mock.Anything, mock.Anything).Return([]float32{0.1, 0.2, 0.3}, nil)

	ragSvc := service.NewRAGService(mockRepo, mockAI, logger)

	results, err := ragSvc.SearchKnowledge(context.Background(), "xyzzy unrelated query words", 2)
	assert.NoError(t, err)
	assert.NotEmpty(t, results, "fallback should still return general docs")
	assert.True(t, results[0].Fallback, "fallback docs must be flagged")
	assert.Equal(t, float32(0), results[0].Score, "fallback docs must not carry a fake relevance score")
}

func TestRAGService_AddKnowledge_Chunking(t *testing.T) {
	mockRepo := new(MockKnowledgeRepo)
	mockAI := new(MockRequestyService)
	logger := zap.NewNop()

	mockRepo.On("ListAll", mock.Anything).Return([]model.FinancialKnowledge{}, nil).Maybe()
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("DeleteByTopic", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("ReplaceByTopic", mock.Anything, "long_topic", mock.Anything).
		Run(func(args mock.Arguments) {
			docs := args.Get(2).([]*model.FinancialKnowledge)
			assert.GreaterOrEqual(t, len(docs), 2, "long content must be split into multiple chunks")
			for _, d := range docs {
				assert.NotEmpty(t, d.Embedding)
				assert.Equal(t, "long_topic", d.Topic)
			}
		}).
		Return(nil).Once()
	// Background seeding goroutine may upsert any embedded knowledge topic
	mockRepo.On("ReplaceByTopic", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockAI.On("IsConfigured").Return(false)

	ragSvc := service.NewRAGService(mockRepo, mockAI, logger)

	longContent := `Đoạn 1: Nội dung chi tiết về phân bổ tài chính và nguyên lý cơ bản của quản lý dòng tiền cá nhân.
Nội dung này rất dài và cần được phân tách thành nhiều đoạn nhỏ để tối ưu hóa quá trình tính vector và tìm kiếm thông tin theo ngữ cảnh.

Đoạn 2: Các bước thực hiện chi tiết trong thực tế bao gồm thiết lập hạn mức ngân sách, theo dõi chi tiêu hàng ngày và cắt giảm các khoản lãng phí.
Mỗi khoản chi tiêu cần được phân loại chính xác vào danh mục thiết yếu hoặc mong muốn để tránh vượt ngân sách.

Đoạn 3: Đánh giá định kỳ cuối tháng và điều chỉnh tỷ lệ phân bổ khi thu nhập thay đổi hoặc khi có các biến cố tài chính phát sinh.`

	err := ragSvc.AddKnowledge(context.Background(), "long_topic", "Cẩm nang Tài chính", longContent)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
