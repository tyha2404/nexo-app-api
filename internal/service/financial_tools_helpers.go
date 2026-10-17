package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"golang.org/x/text/unicode/norm"
)

// toolErrorJSON builds a valid JSON error payload for a tool result (safe
// against quotes in user-provided names).
func toolErrorJSON(msg string) string {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return string(b)
}

// toolNumberArg reads a numeric tool argument. Models sometimes send numbers
// as strings ("50000"), so both are accepted. present is false when the key
// is missing or empty; ok is false when the value is not a finite number.
func toolNumberArg(args map[string]interface{}, key string) (value float64, present bool, ok bool) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return 0, false, true
	}
	switch v := raw.(type) {
	case float64:
		value = v
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, false, true
		}
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, true, false
		}
		value = parsed
	default:
		return 0, true, false
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, true, false
	}
	return value, true, true
}

// toolPositiveAmount reads a required amount that must be a finite number > 0.
func toolPositiveAmount(args map[string]interface{}, key string) (float64, bool) {
	v, present, ok := toolNumberArg(args, key)
	if !present || !ok || v <= 0 {
		return 0, false
	}
	return v, true
}

// toolNonNegativeAmount reads an optional amount (default 0) that must be a
// finite number >= 0 when present.
func toolNonNegativeAmount(args map[string]interface{}, key string) (float64, bool) {
	v, present, ok := toolNumberArg(args, key)
	if !present {
		return 0, true
	}
	if !ok || v < 0 {
		return 0, false
	}
	return v, true
}

// normalizeName lowercases, strips Vietnamese diacritics (đ → d) and
// collapses whitespace so "Ví Tiền Mặt" and "vi tien mat" compare equal.
func normalizeName(s string) string {
	decomposed := norm.NFD.String(strings.ToLower(strings.TrimSpace(s)))
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r == 'đ' {
			r = 'd'
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// walletMatchError explains why a wallet name could not be resolved. Its
// message is shown to the model so it can ask the user to clarify.
type walletMatchError struct {
	query      string
	candidates []string
}

func (e *walletMatchError) Error() string {
	if e.query == "" {
		return fmt.Sprintf("Người dùng chưa nói rõ dùng ví nào. Hãy hỏi lại người dùng muốn dùng ví nào trong các ví: %s.", strings.Join(e.candidates, ", "))
	}
	if len(e.candidates) > 1 {
		return fmt.Sprintf("Tên ví '%s' khớp với nhiều ví: %s. Hãy hỏi lại người dùng để chọn đúng một ví.", e.query, strings.Join(e.candidates, ", "))
	}
	return fmt.Sprintf("Không tìm thấy ví '%s'. Hãy hỏi lại người dùng muốn dùng ví nào trong các ví: %s.", e.query, strings.Join(e.candidates, ", "))
}

func walletNames(wallets []model.Wallet) []string {
	names := make([]string, 0, len(wallets))
	for _, w := range wallets {
		names = append(names, w.Name)
	}
	return names
}

// matchWalletByName resolves a user-supplied wallet name:
//  1. exact match (case/diacritics-insensitive);
//  2. otherwise a unique "contains" match;
//  3. otherwise a *walletMatchError (none or ambiguous).
//
// An empty name resolves only when the user has exactly one wallet. It never
// silently picks the first wallet.
func matchWalletByName(wallets []model.Wallet, name string) (*model.Wallet, error) {
	query := normalizeName(name)
	if query == "" {
		if len(wallets) == 1 {
			return &wallets[0], nil
		}
		return nil, &walletMatchError{candidates: walletNames(wallets)}
	}

	var exact, partial []int
	for i := range wallets {
		wn := normalizeName(wallets[i].Name)
		switch {
		case wn == query:
			exact = append(exact, i)
		case strings.Contains(wn, query):
			partial = append(partial, i)
		}
	}

	pick := exact
	if len(pick) == 0 {
		pick = partial
	}
	if len(pick) == 1 {
		return &wallets[pick[0]], nil
	}

	if len(pick) > 1 {
		names := make([]string, 0, len(pick))
		for _, i := range pick {
			names = append(names, wallets[i].Name)
		}
		return nil, &walletMatchError{query: name, candidates: names}
	}
	return nil, &walletMatchError{query: name, candidates: walletNames(wallets)}
}

// matchCategoryByName prefers a case-insensitive exact name, then the first
// category whose name contains the query (the tools' historical behaviour).
func matchCategoryByName(categories []model.Category, name string) *model.Category {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	for i := range categories {
		if strings.EqualFold(categories[i].Name, name) {
			return &categories[i]
		}
	}
	lower := strings.ToLower(name)
	for i := range categories {
		if strings.Contains(strings.ToLower(categories[i].Name), lower) {
			return &categories[i]
		}
	}
	return nil
}

// matchDebtByTitle prefers an exact title, then a containment match either way.
func matchDebtByTitle(debts []dto.DebtResponse, title string) *dto.DebtResponse {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	for i := range debts {
		if strings.EqualFold(debts[i].Title, title) {
			return &debts[i]
		}
	}
	lower := strings.ToLower(title)
	for i := range debts {
		t := strings.ToLower(debts[i].Title)
		if strings.Contains(t, lower) || strings.Contains(lower, t) {
			return &debts[i]
		}
	}
	return nil
}
