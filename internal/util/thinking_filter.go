package util

import (
	"regexp"
	"strings"
)

var (
	thinkTagRegex   = regexp.MustCompile(`(?is)<think>.*?</think>|<thought>.*?</thought>|<thinking>.*?</thinking>`)
	unclosedTagRegex = regexp.MustCompile(`(?is)<think>.*$|<thought>.*$|<thinking>.*$`)
)

// StripThinkingTags cleans any <think>, <thought>, or <thinking> blocks from a completed string.
func StripThinkingTags(text string) string {
	if text == "" {
		return ""
	}
	cleaned := thinkTagRegex.ReplaceAllString(text, "")
	cleaned = unclosedTagRegex.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

// ThinkingStreamFilter parses and filters thinking tags from streaming token chunks in real time.
type ThinkingStreamFilter struct {
	insideThinking bool
	buffer         string
}

func NewThinkingStreamFilter() *ThinkingStreamFilter {
	return &ThinkingStreamFilter{}
}

var (
	startTags = []string{"<think>", "<thought>", "<thinking>"}
	endTags   = []string{"</think>", "</thought>", "</thinking>"}
)

// Process takes a streaming chunk and returns only the visible text outside thinking tags.
func (f *ThinkingStreamFilter) Process(chunk string) string {
	f.buffer += chunk
	var output strings.Builder

	for len(f.buffer) > 0 {
		lower := strings.ToLower(f.buffer)

		if !f.insideThinking {
			pos := strings.Index(lower, "<")
			if pos == -1 {
				output.WriteString(f.buffer)
				f.buffer = ""
				break
			}

			if pos > 0 {
				output.WriteString(f.buffer[:pos])
				f.buffer = f.buffer[pos:]
				lower = strings.ToLower(f.buffer)
			}

			// Check for exact start tags
			matchedTag := ""
			for _, tag := range startTags {
				if strings.HasPrefix(lower, tag) {
					matchedTag = tag
					break
				}
			}

			if matchedTag != "" {
				f.insideThinking = true
				f.buffer = f.buffer[len(matchedTag):]
				continue
			}

			// Check if buffer is a partial prefix of any start tag
			isPartial := false
			for _, tag := range startTags {
				if strings.HasPrefix(tag, lower) {
					isPartial = true
					break
				}
			}

			if isPartial && len(f.buffer) < 12 {
				// Need more data to determine if it's a start tag
				break
			}

			// Not a think tag start (e.g. "< 500k" or "<div>")
			output.WriteString(f.buffer[:1])
			f.buffer = f.buffer[1:]
		} else {
			// Currently inside thinking mode: search for closing tag
			matchedTag := ""
			matchIndex := -1

			for _, tag := range endTags {
				idx := strings.Index(lower, tag)
				if idx != -1 {
					if matchIndex == -1 || idx < matchIndex {
						matchIndex = idx
						matchedTag = tag
					}
				}
			}

			if matchIndex != -1 {
				// Found end tag
				f.insideThinking = false
				f.buffer = f.buffer[matchIndex+len(matchedTag):]

				// Strip optional leading newline immediately following closing tag
				if len(f.buffer) > 0 && (f.buffer[0] == '\n' || f.buffer[0] == '\r') {
					if len(f.buffer) > 1 && f.buffer[0] == '\r' && f.buffer[1] == '\n' {
						f.buffer = f.buffer[2:]
					} else {
						f.buffer = f.buffer[1:]
					}
				}
				continue
			}

			// Check if buffer ends with a partial prefix of any closing tag
			partialFound := false
			for _, tag := range endTags {
				for i := 1; i < len(tag); i++ {
					prefix := tag[:i]
					if strings.HasSuffix(lower, prefix) {
						// Keep only the suffix in buffer to test against next chunks
						f.buffer = f.buffer[len(f.buffer)-len(prefix):]
						partialFound = true
						break
					}
				}
				if partialFound {
					break
				}
			}

			if partialFound {
				break
			}

			// Inside thinking and no partial tag matched, discard buffer
			f.buffer = ""
			break
		}
	}

	return output.String()
}

// Flush returns any remaining non-thinking buffered characters at the end of the stream.
func (f *ThinkingStreamFilter) Flush() string {
	if !f.insideThinking {
		res := f.buffer
		f.buffer = ""
		return res
	}
	f.buffer = ""
	return ""
}
