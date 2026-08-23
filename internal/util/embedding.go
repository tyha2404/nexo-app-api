package util

import (
	"hash/fnv"
	"math"
	"regexp"
	"strings"
	"unicode"
)

var nonAlphanumericRegex = regexp.MustCompile(`[^\p{L}\p{N}\s]+`)

// GenerateLocalEmbedding produces a deterministic, normalized []float32 embedding vector
// using subword, n-gram, and word feature hashing with L2-normalization.
func GenerateLocalEmbedding(text string, dim int) []float32 {
	if dim <= 0 {
		dim = 256
	}

	vec := make([]float32, dim)
	cleanText := strings.ToLower(strings.TrimSpace(text))
	if cleanText == "" {
		return vec
	}

	// Clean punctuation
	cleanText = nonAlphanumericRegex.ReplaceAllString(cleanText, " ")
	words := strings.Fields(cleanText)
	if len(words) == 0 {
		return vec
	}

	// 1. Unigram word hashing
	for pos, w := range words {
		if len(w) == 0 {
			continue
		}
		weight := float32(1.0)
		if pos < 10 {
			// Boost early tokens / title
			weight = 1.3
		}
		hashAndAccumulate(vec, "w:"+w, weight, dim)
	}

	// 2. Bigram word hashing
	for i := 0; i < len(words)-1; i++ {
		bigram := words[i] + "_" + words[i+1]
		hashAndAccumulate(vec, "b:"+bigram, 1.2, dim)
	}

	// 3. Trigram word hashing
	for i := 0; i < len(words)-2; i++ {
		trigram := words[i] + "_" + words[i+1] + "_" + words[i+2]
		hashAndAccumulate(vec, "t:"+trigram, 1.0, dim)
	}

	// 4. Character n-grams (3-gram and 4-gram) for robust subword/Vietnamese diacritics & typo matching
	runes := []rune(cleanText)
	for i := 0; i <= len(runes)-3; i++ {
		if !unicode.IsSpace(runes[i]) && !unicode.IsSpace(runes[i+1]) && !unicode.IsSpace(runes[i+2]) {
			hashAndAccumulate(vec, "c3:"+string(runes[i:i+3]), 0.6, dim)
		}
	}
	for i := 0; i <= len(runes)-4; i++ {
		if !unicode.IsSpace(runes[i]) && !unicode.IsSpace(runes[i+1]) && !unicode.IsSpace(runes[i+2]) && !unicode.IsSpace(runes[i+3]) {
			hashAndAccumulate(vec, "c4:"+string(runes[i:i+4]), 0.5, dim)
		}
	}

	// 5. L2 Normalization
	var sumSquares float64
	for _, val := range vec {
		sumSquares += float64(val * val)
	}

	if sumSquares > 0 {
		norm := float32(math.Sqrt(sumSquares))
		for i := range vec {
			vec[i] = vec[i] / norm
		}
	}

	return vec
}

func hashAndAccumulate(vec []float32, feature string, weight float32, dim int) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(feature))
	hashVal := h.Sum32()

	index := int(hashVal % uint32(dim))
	vec[index] += weight
}
