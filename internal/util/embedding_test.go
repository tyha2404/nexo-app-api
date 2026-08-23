package util_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

func TestGenerateLocalEmbedding(t *testing.T) {
	dim := 256
	text := "Quy tắc Quản lý Tài chính 50/30/20"
	vec := util.GenerateLocalEmbedding(text, dim)

	assert.Equal(t, dim, len(vec))

	// Ensure non-zero elements
	hasNonZero := false
	var sumSquares float64
	for _, v := range vec {
		if v != 0 {
			hasNonZero = true
		}
		sumSquares += float64(v * v)
	}
	assert.True(t, hasNonZero, "Vector should have non-zero elements")
	assert.InDelta(t, 1.0, math.Sqrt(sumSquares), 0.001, "Vector should be unit length (L2 normalized)")

	// Similar texts should have high similarity
	textSimilar := "Tư vấn phương pháp phân bổ chi tiêu 50/30/20"
	vecSimilar := util.GenerateLocalEmbedding(textSimilar, dim)

	var dotSimilar float32
	for i := range vec {
		dotSimilar += vec[i] * vecSimilar[i]
	}
	assert.True(t, dotSimilar > 0.15, "Similar financial texts should have high cosine similarity, got %f", dotSimilar)

	// Unrelated text should have low similarity
	textUnrelated := "Nấu món cá kho tộ miền Tây thơm ngon đậm đà"
	vecUnrelated := util.GenerateLocalEmbedding(textUnrelated, dim)

	var dotUnrelated float32
	for i := range vec {
		dotUnrelated += vec[i] * vecUnrelated[i]
	}
	assert.True(t, dotSimilar > dotUnrelated, "Similar text similarity (%f) should be higher than unrelated text similarity (%f)", dotSimilar, dotUnrelated)
}
