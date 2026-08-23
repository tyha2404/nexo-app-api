package httpclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/logger"
	"github.com/tyha2404/nexo-app-api/internal/util/httpclient"
)

func TestMaskToken(t *testing.T) {
	assert.Equal(t, "******", httpclient.MaskToken("short"))
	assert.Equal(t, "1234...9012", httpclient.MaskToken("123456789012"))
}

func TestLoggingTransport_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer secret-token-long-string-12345", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	logg, err := logger.New("info", "dev")
	assert.NoError(t, err)

	client := &http.Client{
		Transport: httpclient.NewLoggingTransport("TestService", logg),
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/test", nil)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer secret-token-long-string-12345")

	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLoggingTransport_Error(t *testing.T) {
	logg, err := logger.New("info", "dev")
	assert.NoError(t, err)

	client := &http.Client{
		Transport: httpclient.NewLoggingTransport("TestService", logg),
	}

	// Request to invalid port/closed server to simulate network failure
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:1/nonexistent", nil)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}
