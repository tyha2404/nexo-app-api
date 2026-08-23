package logger

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	Reset       = "\033[0m"
	Bold        = "\033[1m"
	Dim         = "\033[2m"
	Italic      = "\033[3m"
	Underline   = "\033[4m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Blue        = "\033[34m"
	Magenta     = "\033[35m"
	Cyan        = "\033[36m"
	White       = "\033[37m"
	HiBlack     = "\033[90m" // Bright black / gray
	HiRed       = "\033[91m"
	HiGreen     = "\033[92m"
	HiYellow    = "\033[93m"
	HiBlue      = "\033[94m"
	HiMagenta   = "\033[95m"
	HiCyan      = "\033[96m"
	HiWhite     = "\033[97m"
	BgRed       = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow    = "\033[43m"
	BgBlue      = "\033[44m"
	BgMagenta   = "\033[45m"
	BgCyan      = "\033[46m"
	BgHiBlack   = "\033[100m"
	BgHiRed     = "\033[101m"
	BgHiGreen   = "\033[102m"
	BgHiYellow  = "\033[103m"
	BgHiBlue    = "\033[104m"
	BgHiMagenta = "\033[105m"
	BgHiCyan    = "\033[106m"
)

// ColorMethod formats HTTP methods with distinct vibrant background badges
func ColorMethod(method string) string {
	switch method {
	case http.MethodGet:
		return fmt.Sprintf("%s%s%s GET    %s", BgCyan, Bold, White, Reset)
	case http.MethodPost:
		return fmt.Sprintf("%s%s%s POST   %s", BgGreen, Bold, White, Reset)
	case http.MethodPut:
		return fmt.Sprintf("%s%s%s PUT    %s", BgYellow, Bold, White, Reset)
	case http.MethodPatch:
		return fmt.Sprintf("%s%s%s PATCH  %s", BgMagenta, Bold, White, Reset)
	case http.MethodDelete:
		return fmt.Sprintf("%s%s%s DELETE %s", BgRed, Bold, White, Reset)
	case http.MethodOptions:
		return fmt.Sprintf("%s%s%s OPTION %s", BgHiBlack, Bold, White, Reset)
	case http.MethodHead:
		return fmt.Sprintf("%s%s%s HEAD   %s", BgBlue, Bold, White, Reset)
	default:
		return fmt.Sprintf("%s%s%s %-6s %s", BgBlue, Bold, White, method, Reset)
	}
}

// ColorStatus formats HTTP status code with descriptive label and color badge
func ColorStatus(status int) string {
	switch status {
	case http.StatusOK:
		return fmt.Sprintf("%s%s%s 200 OK       %s", BgGreen, Bold, White, Reset)
	case http.StatusCreated:
		return fmt.Sprintf("%s%s%s 201 CREATED  %s", BgHiGreen, Bold, White, Reset)
	case http.StatusAccepted:
		return fmt.Sprintf("%s%s%s 202 ACCEPTED %s", BgGreen, Bold, White, Reset)
	case http.StatusNoContent:
		return fmt.Sprintf("%s%s%s 204 NO CONT  %s", BgGreen, Bold, White, Reset)
	case http.StatusMovedPermanently:
		return fmt.Sprintf("%s%s%s 301 MOVED    %s", BgCyan, Bold, White, Reset)
	case http.StatusFound:
		return fmt.Sprintf("%s%s%s 302 FOUND    %s", BgCyan, Bold, White, Reset)
	case http.StatusNotModified:
		return fmt.Sprintf("%s%s%s 304 NOT MOD  %s", BgCyan, Bold, White, Reset)
	case http.StatusBadRequest:
		return fmt.Sprintf("%s%s%s 400 BAD REQ  %s", BgYellow, Bold, White, Reset)
	case http.StatusUnauthorized:
		return fmt.Sprintf("%s%s%s 401 UNAUTH   %s", BgHiMagenta, Bold, White, Reset)
	case http.StatusForbidden:
		return fmt.Sprintf("%s%s%s 403 FORBID   %s", BgHiYellow, Bold, White, Reset)
	case http.StatusNotFound:
		return fmt.Sprintf("%s%s%s 404 NOT FOUND%s", BgYellow, Bold, White, Reset)
	case http.StatusMethodNotAllowed:
		return fmt.Sprintf("%s%s%s 405 METHOD NA%s", BgYellow, Bold, White, Reset)
	case http.StatusConflict:
		return fmt.Sprintf("%s%s%s 409 CONFLICT %s", BgHiYellow, Bold, White, Reset)
	case http.StatusUnprocessableEntity:
		return fmt.Sprintf("%s%s%s 422 UNPROC   %s", BgYellow, Bold, White, Reset)
	case http.StatusTooManyRequests:
		return fmt.Sprintf("%s%s%s 429 RATELIM  %s", BgHiRed, Bold, White, Reset)
	case http.StatusInternalServerError:
		return fmt.Sprintf("%s%s%s 500 ERR      %s", BgRed, Bold, White, Reset)
	case http.StatusBadGateway:
		return fmt.Sprintf("%s%s%s 502 BAD GATE %s", BgRed, Bold, White, Reset)
	case http.StatusServiceUnavailable:
		return fmt.Sprintf("%s%s%s 503 UNAVAIL  %s", BgRed, Bold, White, Reset)
	case http.StatusGatewayTimeout:
		return fmt.Sprintf("%s%s%s 504 TIMEOUT  %s", BgRed, Bold, White, Reset)
	default:
		switch {
		case status >= 200 && status < 300:
			return fmt.Sprintf("%s%s%s %d SUCCESS %s", BgGreen, Bold, White, status, Reset)
		case status >= 300 && status < 400:
			return fmt.Sprintf("%s%s%s %d REDIR   %s", BgCyan, Bold, White, status, Reset)
		case status >= 400 && status < 500:
			return fmt.Sprintf("%s%s%s %d CLIENT  %s", BgYellow, Bold, White, status, Reset)
		default:
			return fmt.Sprintf("%s%s%s %d ERROR   %s", BgRed, Bold, White, status, Reset)
		}
	}
}

// ColorDuration returns execution time formatted with fine-grained color thresholds
func ColorDuration(d time.Duration) string {
	ms := float64(d.Microseconds()) / 1000.0
	switch {
	case d < 10*time.Millisecond:
		return fmt.Sprintf("%s%s%7.2fms%s", HiGreen, Bold, ms, Reset)
	case d < 50*time.Millisecond:
		return fmt.Sprintf("%s%7.2fms%s", Green, ms, Reset)
	case d < 200*time.Millisecond:
		return fmt.Sprintf("%s%7.2fms%s", Yellow, ms, Reset)
	default:
		return fmt.Sprintf("%s%s%7.2fms%s", HiRed, Bold, ms, Reset)
	}
}

// ColorURL separates API prefix, main resource endpoint, and query string in distinct colors
func ColorURL(path string, queryString string) string {
	var sb strings.Builder

	if strings.HasPrefix(path, "/api/v1") {
		sb.WriteString(fmt.Sprintf("%s/api/v1%s", HiBlack, Reset))
		rest := strings.TrimPrefix(path, "/api/v1")
		if rest == "" {
			sb.WriteString(fmt.Sprintf("%s/%s", Bold+White, Reset))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s%s", Bold+HiWhite, rest, Reset))
		}
	} else if strings.HasPrefix(path, "/swagger") {
		sb.WriteString(fmt.Sprintf("%s%s%s", Cyan, path, Reset))
	} else {
		sb.WriteString(fmt.Sprintf("%s%s%s", Bold+White, path, Reset))
	}

	if queryString != "" {
		sb.WriteString(fmt.Sprintf("%s?%s%s", Yellow, queryString, Reset))
	}

	return sb.String()
}

// ColorBytes formats byte size with human readable units
func ColorBytes(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%s%4d B%s", HiBlack, bytes, Reset)
	case bytes < 1024*1024:
		return fmt.Sprintf("%s%5.1f KB%s", Cyan, float64(bytes)/1024.0, Reset)
	default:
		return fmt.Sprintf("%s%5.1f MB%s", Magenta, float64(bytes)/(1024.0*1024.0), Reset)
	}
}

// ColorIP formats IP and port
func ColorIP(ip string) string {
	return fmt.Sprintf("%s%s%s", HiBlack, ip, Reset)
}

var (
	sqlKeywordsRegex    = regexp.MustCompile(`(?i)\b(SELECT|FROM|WHERE|INSERT INTO|INSERT|UPDATE|DELETE FROM|DELETE|SET|VALUES|JOIN|LEFT JOIN|RIGHT JOIN|INNER JOIN|FULL JOIN|CROSS JOIN|ON|GROUP BY|ORDER BY|HAVING|LIMIT|OFFSET|AND|OR|NOT|IN|LIKE|ILIKE|IS NULL|IS NOT NULL|RETURNING|UNION ALL|UNION|EXISTS|BETWEEN|CASE|WHEN|THEN|ELSE|END|AS|ASC|DESC|DISTINCT|COUNT|SUM|AVG|MIN|MAX|TRUE|FALSE|NULL)\b`)
	sqlStringRegex      = regexp.MustCompile(`'([^'\\]|\\.)*'`)
	sqlPlaceholderRegex = regexp.MustCompile(`(\$\d+|\?)`)
)

// HighlightSQL formats the entire SQL query in vivid syntax colors (non-default colors for everything)
func HighlightSQL(sql string) string {
	baseColor := HiYellow // Warm vibrant yellow for columns, tables, punctuation

	// 1. Highlight SQL string literals in vibrant green
	res := sqlStringRegex.ReplaceAllStringFunc(sql, func(str string) string {
		return fmt.Sprintf("%s%s%s%s", HiGreen, str, Reset, baseColor)
	})

	// 2. Highlight SQL placeholders ($1, $2, ?) in bold bright magenta
	res = sqlPlaceholderRegex.ReplaceAllStringFunc(res, func(ph string) string {
		return fmt.Sprintf("%s%s%s%s%s", Bold, HiMagenta, ph, Reset, baseColor)
	})

	// 3. Highlight SQL keywords in bold bright cyan
	res = sqlKeywordsRegex.ReplaceAllStringFunc(res, func(kw string) string {
		return fmt.Sprintf("%s%s%s%s%s", Bold, HiCyan, strings.ToUpper(kw), Reset, baseColor)
	})

	return fmt.Sprintf("%s%s%s", baseColor, res, Reset)
}
