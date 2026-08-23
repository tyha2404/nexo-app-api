package util

import (
	"strings"
	"unicode/utf8"
)

// TextChunk represents a segmented chunk of a document
type TextChunk struct {
	Index   int    `json:"index"`
	Total   int    `json:"total"`
	Content string `json:"content"`
}

// ChunkOptions configures the recursive text chunker
type ChunkOptions struct {
	MaxChunkSize int // Maximum number of characters per chunk (default: 450)
	Overlap      int // Overlap in characters between adjacent chunks (default: 80)
}

// DefaultChunkOptions provides standard production settings for RAG
var DefaultChunkOptions = ChunkOptions{
	MaxChunkSize: 450,
	Overlap:      80,
}

// ChunkText splits long documents into semantic, overlapping chunks using recursive boundaries.
func ChunkText(text string, opts ...ChunkOptions) []TextChunk {
	opt := DefaultChunkOptions
	if len(opts) > 0 {
		if opts[0].MaxChunkSize > 0 {
			opt.MaxChunkSize = opts[0].MaxChunkSize
		}
		if opts[0].Overlap >= 0 {
			opt.Overlap = opts[0].Overlap
		}
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	if utf8.RuneCountInString(trimmed) <= opt.MaxChunkSize {
		return []TextChunk{
			{
				Index:   0,
				Total:   1,
				Content: trimmed,
			},
		}
	}

	// Recursive split hierarchy: Paragraphs -> Lines -> Sentences -> Words
	rawChunks := recursiveSplit(trimmed, []string{"\n\n", "\n", ". ", "; ", ", ", " "}, opt.MaxChunkSize, opt.Overlap)

	// Format into TextChunk structs with Total count
	result := make([]TextChunk, len(rawChunks))
	for i, c := range rawChunks {
		result[i] = TextChunk{
			Index:   i,
			Total:   len(rawChunks),
			Content: c,
		}
	}

	return result
}

func recursiveSplit(text string, separators []string, maxSize, overlap int) []string {
	var chunks []string
	text = strings.TrimSpace(text)
	if text == "" {
		return chunks
	}

	if utf8.RuneCountInString(text) <= maxSize {
		return []string{text}
	}

	if len(separators) == 0 {
		// Hard cut fallback by runes if no separator matches
		runes := []rune(text)
		for i := 0; i < len(runes); {
			end := i + maxSize
			if end > len(runes) {
				end = len(runes)
			}
			chunk := strings.TrimSpace(string(runes[i:end]))
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
			if end == len(runes) {
				break
			}
			i += maxSize - overlap
			if i <= 0 || i >= len(runes) {
				break
			}
		}
		return chunks
	}

	sep := separators[0]
	splits := strings.Split(text, sep)

	var current strings.Builder
	for i, part := range splits {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Re-append separator if it's not the end
		segment := part
		if i < len(splits)-1 && sep != " " {
			segment += sep
		}

		currentRunes := utf8.RuneCountInString(current.String())
		segmentRunes := utf8.RuneCountInString(segment)

		if currentRunes+segmentRunes <= maxSize {
			if current.Len() > 0 && !strings.HasSuffix(current.String(), "\n") && sep == " " {
				current.WriteString(" ")
			}
			current.WriteString(segment)
		} else {
			if current.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(current.String()))
			}

			// If a single segment is larger than maxSize, recurse with next finer separator
			if segmentRunes > maxSize {
				subChunks := recursiveSplit(part, separators[1:], maxSize, overlap)
				chunks = append(chunks, subChunks...)
				current.Reset()
			} else {
				// Initialize new chunk with overlap from end of previous text
				current.Reset()
				if overlap > 0 && len(chunks) > 0 {
					lastChunk := chunks[len(chunks)-1]
					lastRunes := []rune(lastChunk)
					if len(lastRunes) > overlap {
						overlapText := string(lastRunes[len(lastRunes)-overlap:])
						// Find first space in overlap to not cut words
						if firstSpace := strings.Index(overlapText, " "); firstSpace != -1 && firstSpace < len(overlapText)-1 {
							overlapText = overlapText[firstSpace+1:]
						}
						current.WriteString(overlapText + " ")
					}
				}
				current.WriteString(segment)
			}
		}
	}

	if current.Len() > 0 {
		trimmedChunk := strings.TrimSpace(current.String())
		if trimmedChunk != "" {
			chunks = append(chunks, trimmedChunk)
		}
	}

	return chunks
}
