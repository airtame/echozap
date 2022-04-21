package echozap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type logLevelTest struct {
	httpStatus       int
	expectedLogLevel zapcore.Level
}

func TestFields(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/something", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := func(c echo.Context) error {
		return c.String(http.StatusOK, "")
	}

	obs, logs := observer.New(zap.DebugLevel)

	logger := zap.New(obs)

	err := ZapLogger(logger)(h)(c)

	assert.Nil(t, err)

	logEntry := logs.AllUntimed()[0]
	logFields := logEntry.ContextMap()

	assert.Equal(t, 1, logs.Len())
	assert.Equal(t, int64(200), logFields["status"])
	assert.Equal(t, "example.com", logFields["host"])
	assert.NotNil(t, logFields["latency"])
	assert.NotNil(t, logFields["latency_human"])
	assert.Equal(t, "GET", logFields["method"])
	assert.Equal(t, "/something", logFields["uri"])
	assert.NotNil(t, logFields["host"])
	assert.Equal(t, int64(0), logFields["bytes_in"])
	assert.Equal(t, int64(0), logFields["bytes_out"])
	assert.Equal(t, zapcore.InfoLevel, logEntry.Level)
}

func TestDefaultLogLevels(t *testing.T) {
	tests := []logLevelTest{
		{100, zapcore.InfoLevel},
		{101, zapcore.InfoLevel},
		{200, zapcore.InfoLevel},
		{201, zapcore.InfoLevel},
		{300, zapcore.InfoLevel},
		{301, zapcore.InfoLevel},
		{400, zapcore.WarnLevel},
		{401, zapcore.WarnLevel},
		{500, zapcore.ErrorLevel},
		{501, zapcore.ErrorLevel},
	}
	testLogLevels(t, DefaultConfig, tests)
}

func TestCustomLogLevels(t *testing.T) {
	config := DefaultConfig
	config.LogLevel = func(status int) zapcore.Level {
		switch {
		case status >= 500:
			return zapcore.DPanicLevel
		case status >= 400:
			return zapcore.InfoLevel
		default:
			return zapcore.DebugLevel
		}
	}
	tests := []logLevelTest{
		{200, zapcore.DebugLevel},
		{400, zapcore.InfoLevel},
		{500, zapcore.DPanicLevel},
	}
	testLogLevels(t, config, tests)
}

func testLogLevels(t *testing.T, config Config, tests []logLevelTest) {
	for _, test := range tests {
		handler := func(c echo.Context) error {
			return c.NoContent(test.httpStatus)
		}

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/something", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		obs, logs := observer.New(zap.DebugLevel)
		logger := zap.New(obs)
		err := ZapLoggerWithConfig(logger, config)(handler)(c)
		assert.Nil(t, err)

		logEntry := logs.AllUntimed()[0]
		assert.Equal(t, test.expectedLogLevel, logEntry.Level)
	}
}
