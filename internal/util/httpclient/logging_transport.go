package httpclient

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/logger"
	"go.uber.org/zap"
)

type LoggingTransport struct {
	ServiceName string
	Transport   http.RoundTripper
	Logger      *zap.Logger
}

func NewLoggingTransport(serviceName string, logg *zap.Logger) *LoggingTransport {
	return &LoggingTransport{
		ServiceName: serviceName,
		Transport:   http.DefaultTransport,
		Logger:      logg,
	}
}

// MaskToken masks a token leaving only first and last 4 characters visible
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "******"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func formatURL(rawURL string) string {
	// Separate protocol://host and path/query
	parts := strings.SplitN(rawURL, "://", 2)
	if len(parts) < 2 {
		return rawURL
	}
	proto := parts[0]
	rest := parts[1]
	slashIdx := strings.Index(rest, "/")
	if slashIdx == -1 {
		return fmt.Sprintf("%s%s://%s%s%s", logger.Dim, proto, logger.Bold+logger.HiWhite, rest, logger.Reset)
	}
	host := rest[:slashIdx]
	pathAndQuery := rest[slashIdx:]
	return fmt.Sprintf("%s%s://%s%s%s%s%s",
		logger.Dim, proto,
		logger.Bold+logger.HiCyan, host, logger.Reset,
		logger.White, pathAndQuery,
	)
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	nowStr := fmt.Sprintf("%s%s%s", logger.HiBlack, start.Format("15:04:05.000"), logger.Reset)

	// Mask Authorization header if present
	authHeader := req.Header.Get("Authorization")
	maskedAuth := ""
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			maskedAuth = "Bearer " + MaskToken(strings.TrimPrefix(authHeader, "Bearer "))
		} else {
			maskedAuth = MaskToken(authHeader)
		}
	}

	// Outbound request console log
	authSuffix := ""
	if maskedAuth != "" {
		authSuffix = fmt.Sprintf(" %s[%s]%s", logger.HiBlack, maskedAuth, logger.Reset)
	}

	serviceBadge := fmt.Sprintf("%s%s %s %s", logger.BgBlue, logger.Bold+logger.White, t.ServiceName, logger.Reset)
	reqBadge := fmt.Sprintf("%s%s EXT ➔ OUT %s", logger.BgMagenta, logger.Bold+logger.White, logger.Reset)

	fmt.Printf("%s %s %s | %s %s%s\n",
		reqBadge,
		serviceBadge,
		nowStr,
		logger.ColorMethod(req.Method),
		formatURL(req.URL.String()),
		authSuffix,
	)

	resp, err := t.Transport.RoundTrip(req)
	duration := time.Since(start)
	respNowStr := fmt.Sprintf("%s%s%s", logger.HiBlack, time.Now().Format("15:04:05.000"), logger.Reset)

	if err != nil {
		errBadge := fmt.Sprintf("%s%s EXT ✖ ERR %s", logger.BgRed, logger.Bold+logger.White, logger.Reset)
		fmt.Printf("%s %s %s | %s | %s %s | %sError: %v%s\n",
			errBadge,
			serviceBadge,
			respNowStr,
			logger.ColorDuration(duration),
			logger.ColorMethod(req.Method),
			formatURL(req.URL.String()),
			logger.Red+logger.Bold, err, logger.Reset,
		)
		if t.Logger != nil {
			t.Logger.Error("External API Request Failed",
				zap.String("service", t.ServiceName),
				zap.String("method", req.Method),
				zap.String("url", req.URL.String()),
				zap.Duration("duration", duration),
				zap.Error(err),
			)
		}
		return nil, err
	}

	// Outbound response console log
	respBadge := fmt.Sprintf("%s%s EXT 🠐 IN  %s", logger.BgHiCyan, logger.Bold+logger.White, logger.Reset)
	fmt.Printf("%s %s %s | %s | %s | %s %s\n",
		respBadge,
		serviceBadge,
		respNowStr,
		logger.ColorStatus(resp.StatusCode),
		logger.ColorDuration(duration),
		logger.ColorMethod(req.Method),
		formatURL(req.URL.String()),
	)

	if resp.StatusCode >= 400 && t.Logger != nil {
		t.Logger.Warn("External API Non-2xx Response",
			zap.String("service", t.ServiceName),
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Int("status", resp.StatusCode),
			zap.Duration("duration", duration),
		)
	}

	return resp, nil
}
