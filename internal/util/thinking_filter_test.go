package util_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

func TestStripThinkingTags(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Complete think tag",
			input:    "<think>\nThis is internal reasoning.\nI will calculate 1 + 1.\n</think>\nKết quả là 2.",
			expected: "Kết quả là 2.",
		},
		{
			name:     "Complete thought tag",
			input:    "<thought>\nLet's analyze user spending.\n</thought>\nSố dư hiện tại của bạn là 5.000.000 ₫.",
			expected: "Số dư hiện tại của bạn là 5.000.000 ₫.",
		},
		{
			name:     "Unclosed think tag",
			input:    "<think>\nThis is truncated reasoning without close tag",
			expected: "",
		},
		{
			name:     "No think tag",
			input:    "Chào bạn, tôi có thể giúp gì cho bạn?",
			expected: "Chào bạn, tôi có thể giúp gì cho bạn?",
		},
		{
			name:     "Text with less than sign",
			input:    "Chi tiêu < 500.000 ₫ và > 100.000 ₫.",
			expected: "Chi tiêu < 500.000 ₫ và > 100.000 ₫.",
		},
		{
			name:     "Multiple think blocks",
			input:    "<think>thought 1</think>Hello <think>thought 2</think>World",
			expected: "Hello World",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := util.StripThinkingTags(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestThinkingStreamFilter(t *testing.T) {
	testCases := []struct {
		name     string
		chunks   []string
		expected string
	}{
		{
			name: "Normal streaming with think block in separate chunks",
			chunks: []string{
				"<think>",
				"\nAnalyzing user request...\n",
				"Step 1: check balance\n",
				"</think>",
				"\nXin chào bạn! ",
				"Số dư của bạn là 10.000.000 ₫.",
			},
			expected: "Xin chào bạn! Số dư của bạn là 10.000.000 ₫.",
		},
		{
			name: "Think tag split across chunk boundaries",
			chunks: []string{
				"<thi",
				"nk>",
				"internal thoughts...",
				"</th",
				"ink>",
				"Câu trả lời hoàn chỉnh.",
			},
			expected: "Câu trả lời hoàn chỉnh.",
		},
		{
			name: "Chunks containing less-than comparisons",
			chunks: []string{
				"Giao dịch ",
				"< 500",
				"k được ",
				"<th",
				"ought>thinking</thought>",
				"tìm thấy.",
			},
			expected: "Giao dịch < 500k được tìm thấy.",
		},
		{
			name: "No think tags at all",
			chunks: []string{
				"Hôm nay ",
				"bạn đã chi tiêu ",
				"150.000 ₫.",
			},
			expected: "Hôm nay bạn đã chi tiêu 150.000 ₫.",
		},
		{
			name: "Only thinking, no actual text",
			chunks: []string{
				"<think>Only thinking process</think>",
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := util.NewThinkingStreamFilter()
			var out strings.Builder

			for _, chunk := range tc.chunks {
				processed := filter.Process(chunk)
				out.WriteString(processed)
			}
			out.WriteString(filter.Flush())

			assert.Equal(t, tc.expected, strings.TrimSpace(out.String()))
		})
	}
}
