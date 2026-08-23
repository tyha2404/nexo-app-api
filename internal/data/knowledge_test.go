package data_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/data"
)

func TestLoadEmbeddedKnowledgeDocs(t *testing.T) {
	docs, err := data.LoadEmbeddedKnowledgeDocs()
	assert.NoError(t, err)
	assert.True(t, len(docs) >= 6, "Should load at least 6 embedded knowledge docs")

	found503020 := false
	for _, doc := range docs {
		assert.NotEmpty(t, doc.Topic)
		assert.NotEmpty(t, doc.Title)
		assert.NotEmpty(t, doc.Content)
		if doc.Topic == "50_30_20_rule" {
			found503020 = true
		}
	}
	assert.True(t, found503020, "Should contain 50_30_20_rule doc")
}
