package data

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed knowledge/*.json
var knowledgeFS embed.FS

// KnowledgeDocument represents a structured financial knowledge entry
type KnowledgeDocument struct {
	Topic   string `json:"topic"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// LoadEmbeddedKnowledgeDocs reads all .json files from the embedded knowledge folder
func LoadEmbeddedKnowledgeDocs() ([]KnowledgeDocument, error) {
	var docs []KnowledgeDocument

	entries, err := fs.ReadDir(knowledgeFS, "knowledge")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded knowledge dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := knowledgeFS.ReadFile("knowledge/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", entry.Name(), err)
		}

		var doc KnowledgeDocument
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", entry.Name(), err)
		}

		if doc.Topic != "" && doc.Title != "" {
			docs = append(docs, doc)
		}
	}

	return docs, nil
}
