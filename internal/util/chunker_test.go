package util_test

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

func TestChunkText_ShortText(t *testing.T) {
	short := "Quy tắc 50/30/20 là phương pháp quản lý tài chính phổ biến."
	chunks := util.ChunkText(short, util.ChunkOptions{MaxChunkSize: 200, Overlap: 30})

	assert.Len(t, chunks, 1)
	assert.Equal(t, 0, chunks[0].Index)
	assert.Equal(t, 1, chunks[0].Total)
	assert.Equal(t, short, chunks[0].Content)
}

func TestChunkText_LongTextWithParagraphs(t *testing.T) {
	longText := `Quy tắc 50/30/20 là phương pháp phân bổ thu nhập ròng hàng tháng đơn giản và hiệu quả nhất:
1. 50% Nhu cầu thiết yếu (Needs): Chi trả cho tiền thuê nhà, tiền ăn uống cơ bản, điện nước internet, xăng xe di chuyển, bảo hiểm y tế và các hóa đơn bắt buộc.
2. 30% Mong muốn cá nhân (Wants): Dành cho ăn ngoài nhà hàng, cafe gặp gỡ bạn bè, mua sắm quần áo, giải trí xem phim, du lịch, đăng ký dịch vụ theo nhu cầu cá nhân.
3. 20% Tiết kiệm & Đầu tư (Savings/Debts): Trả nợ thẻ tín dụng/khoản vay, trích lập quỹ khẩn cấp, tích lũy mua tài sản sinh lời (chứng khoán, vàng, gửi tiết kiệm).
Nếu chi phí thiết yếu vượt quá 50%, cần rà soát cắt giảm nhu cầu mong muốn (Wants) hoặc gia tăng thu nhập.`

	chunks := util.ChunkText(longText, util.ChunkOptions{MaxChunkSize: 250, Overlap: 50})

	assert.True(t, len(chunks) >= 2, "Long text should be split into 2 or more chunks")
	for i, c := range chunks {
		assert.Equal(t, i, c.Index)
		assert.Equal(t, len(chunks), c.Total)
		assert.NotEmpty(t, c.Content)
		assert.True(t, utf8.RuneCountInString(c.Content) <= 350, "Chunk size should be within reasonable bounds")
	}
}

func TestChunkText_Empty(t *testing.T) {
	chunks := util.ChunkText("   ")
	assert.Nil(t, chunks)
}
