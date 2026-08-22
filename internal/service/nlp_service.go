package service

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type NLPService interface {
	ParseTransaction(ctx context.Context, userID uuid.UUID, text string) (*dto.ParseNLPResponse, error)
}

type nlpService struct {
	categoryRepo repository.CategoryRepo
}

func NewNLPService(categoryRepo repository.CategoryRepo) NLPService {
	return &nlpService{
		categoryRepo: categoryRepo,
	}
}

// Keywords for income identification
var incomeKeywords = []string{
	"lương", "thưởng", "thu nhập", "nhận", "tiền thưởng", "hoàn tiền", "lương tháng",
}

// Pre-defined mapping from category names to keywords
var categoryKeywordMap = []struct {
	Category string
	Keywords []string
}{
	{
		Category: "Lương",
		Keywords: []string{"lương", "thưởng", "thu nhập", "nhận lương", "tăng ca"},
	},
	{
		Category: "Di chuyển",
		Keywords: []string{"xăng", "xe", "grab", "be", "gobi", "taxi", "gửi xe", "vé xe", "sửa xe"},
	},
	{
		Category: "Hóa đơn",
		Keywords: []string{"tiền nhà", "tiền điện", "tiền nước", "mạng", "internet", "wifi", "điện thoại", "tiền phòng"},
	},
	{
		Category: "Ăn uống",
		Keywords: []string{"ăn", "phở", "cà phê", "cafe", "bún", "cơm", "trà sữa", "nhậu", "tiệc", "uống", "bánh"},
	},
}

func (s *nlpService) ParseTransaction(ctx context.Context, userID uuid.UUID, text string) (*dto.ParseNLPResponse, error) {
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return &dto.ParseNLPResponse{
			Type:            "EXPENSE",
			ConfidenceScore: 0,
		}, nil
	}

	amount, matchRange := parseAmount(trimmedText)

	// Determine transaction type
	lowerText := strings.ToLower(trimmedText)
	txType := string(model.CategoryTypeExpense)
	for _, kw := range incomeKeywords {
		if strings.Contains(lowerText, kw) {
			txType = string(model.CategoryTypeIncome)
			break
		}
	}

	// Clean description by removing extracted amount substring if present
	description := trimmedText
	if matchRange[0] != -1 && matchRange[1] != -1 {
		before := trimmedText[:matchRange[0]]
		after := trimmedText[matchRange[1]:]
		description = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(before+" "+after, " "))
	}
	if description == "" {
		description = trimmedText
	}

	// Category matching
	var matchedCategory *model.Category
	var matchedCategoryName string

	// Fetch user's categories if repository is provided and userID is valid
	var userCategories []model.Category
	if s.categoryRepo != nil && userID != uuid.Nil {
		cats, err := s.categoryRepo.List(ctx, userID, "", 0, 0)
		if err == nil {
			userCategories = cats
		}
	}

	// 1. Try matching against existing user category names
	for i := range userCategories {
		catNameLower := strings.ToLower(userCategories[i].Name)
		if catNameLower != "" && strings.Contains(lowerText, catNameLower) {
			matchedCategory = &userCategories[i]
			matchedCategoryName = userCategories[i].Name
			break
		}
	}

	// 2. If no user category matched, check default category keyword map
	if matchedCategoryName == "" {
		for _, item := range categoryKeywordMap {
			for _, kw := range item.Keywords {
				if strings.Contains(lowerText, kw) {
					matchedCategoryName = item.Category
					// If user has a category with this matched name, attach its ID
					for i := range userCategories {
						if strings.EqualFold(userCategories[i].Name, item.Category) {
							matchedCategory = &userCategories[i]
							break
						}
					}
					break
				}
			}
			if matchedCategoryName != "" {
				break
			}
		}
	}

	// Calculate confidence score
	confidenceScore := 0.5
	if amount > 0 {
		confidenceScore += 0.3
	}
	if matchedCategoryName != "" {
		confidenceScore += 0.2
	}
	if confidenceScore > 1.0 {
		confidenceScore = 1.0
	}

	var categoryID *uuid.UUID
	if matchedCategory != nil {
		categoryID = &matchedCategory.ID
	}

	return &dto.ParseNLPResponse{
		Amount:          amount,
		CategoryID:      categoryID,
		CategoryName:    matchedCategoryName,
		Type:            txType,
		Description:     description,
		ConfidenceScore: confidenceScore,
	}, nil
}

type amountSuffixRule struct {
	Suffix     string
	Multiplier float64
}

var suffixes = []amountSuffixRule{
	{Suffix: "nghìn", Multiplier: 1000},
	{Suffix: "nghin", Multiplier: 1000},
	{Suffix: "triệu", Multiplier: 1000000},
	{Suffix: "trieu", Multiplier: 1000000},
	{Suffix: "tr", Multiplier: 1000000},
	{Suffix: "củ", Multiplier: 1000000},
	{Suffix: "cu", Multiplier: 1000000},
	{Suffix: "k", Multiplier: 1000},
	{Suffix: "m", Multiplier: 1000000},
	{Suffix: "n", Multiplier: 1000},
	{Suffix: "đ", Multiplier: 1},
	{Suffix: "d", Multiplier: 1},
}

var numberRegex = regexp.MustCompile(`(\d+(?:[.,]\d+)?)`)

func parseAmount(text string) (float64, [2]int) {
	noMatch := [2]int{-1, -1}
	lowerText := strings.ToLower(text)

	matches := numberRegex.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return 0, noMatch
	}

	for _, loc := range matches {
		numStart, numEnd := loc[0], loc[1]
		numStr := text[numStart:numEnd]

		// Check suffix immediately following numEnd
		afterNum := lowerText[numEnd:]
		trimmedAfter := strings.TrimLeft(afterNum, " ")
		spacesLen := len(afterNum) - len(trimmedAfter)

		var matchedSuffix string
		var multiplier float64 = 1
		suffixLen := 0

		for _, s := range suffixes {
			if strings.HasPrefix(trimmedAfter, s.Suffix) {
				// Check word boundary after suffix
				afterSuffix := trimmedAfter[len(s.Suffix):]
				if len(afterSuffix) == 0 || regexp.MustCompile(`^[^\p{L}\p{N}]`).MatchString(afterSuffix) {
					matchedSuffix = s.Suffix
					multiplier = s.Multiplier
					suffixLen = len(s.Suffix)
					break
				}
			}
		}

		fullEnd := numEnd
		if matchedSuffix != "" {
			fullEnd = numEnd + spacesLen + suffixLen
		}

		numVal, err := parseFormattedNumber(numStr, matchedSuffix != "" && multiplier > 1)
		if err != nil {
			continue
		}

		amount := numVal * multiplier
		if amount > 0 {
			return amount, [2]int{numStart, fullEnd}
		}
	}

	return 0, noMatch
}

func parseFormattedNumber(numStr string, hasMultiplier bool) (float64, error) {
	if strings.Contains(numStr, ".") && strings.Contains(numStr, ",") {
		lastDot := strings.LastIndex(numStr, ".")
		lastComma := strings.LastIndex(numStr, ",")
		if lastDot > lastComma {
			numStr = strings.ReplaceAll(numStr, ",", "")
		} else {
			numStr = strings.ReplaceAll(numStr, ".", "")
			numStr = strings.ReplaceAll(numStr, ",", ".")
		}
	} else if strings.Contains(numStr, ",") {
		parts := strings.Split(numStr, ",")
		if hasMultiplier || (len(parts) == 2 && len(parts[1]) <= 2) {
			numStr = strings.ReplaceAll(numStr, ",", ".")
		} else {
			numStr = strings.ReplaceAll(numStr, ",", "")
		}
	} else if strings.Contains(numStr, ".") {
		parts := strings.Split(numStr, ".")
		if hasMultiplier {
			// keep dot for decimal, e.g., 1.5tr
		} else {
			// e.g. 50.000
			if len(parts) == 2 && len(parts[1]) == 3 {
				numStr = strings.ReplaceAll(numStr, ".", "")
			}
		}
	}

	return strconv.ParseFloat(numStr, 64)
}
