package echozap

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/bytes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/ioutil"
	"strconv"
	"time"
)

type (
	Config struct {
		// Skipper defines a function to skip middleware.
		Skipper Skipper

		// ContextKeys defines the keys which should be added to the logger, as fields, from the context.
		ContextKeys []interface{}

		// PrintBody defines if the body of the request should be printed, if it exists.
		PrintBody bool
	}

	Skipper func(echo.Context) bool
)

var (
	DefaultConfig = Config{
		Skipper:     DefaultSkipper,
		ContextKeys: nil,
		PrintBody:   true,
	}
)

// ZapLogger is a middleware and zap to provide an "access log" like logging for each request.
func ZapLogger(log *zap.Logger) echo.MiddlewareFunc {
	return ZapLoggerWithConfig(log, DefaultConfig)
}

func ZapLoggerWithConfig(log *zap.Logger, config Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			if config.Skipper(c) {
				return err
			}

			if config.ContextKeys != nil {
				extendWithCtx(c.Request().Context(), log, config.ContextKeys)
			}

			req := c.Request()
			res := c.Response()

			fields := []zapcore.Field{
				zap.String("time", time.Now().Format(time.RFC3339Nano)),
				zap.String("remote_ip", c.RealIP()),
				zap.String("host", req.Host),
				zap.String("method", req.Method),
				zap.String("uri", req.RequestURI),
				zap.String("user_agent", req.UserAgent()),
				zap.Int("status", res.Status),
				zap.Int64("latency", time.Since(start).Nanoseconds()),
				zap.String("latency_human", time.Since(start).String()),
			}

			headerContentLengthRaw := req.Header.Get(echo.HeaderContentLength)
			headerContentLength, parseErr := strconv.ParseInt(headerContentLengthRaw, 10, 64)
			if parseErr != nil {
				headerContentLength = 0
			}
			fields = append(fields, zap.Int64("bytes_in", headerContentLength))
			fields = append(fields, zap.Int64("bytes_out", res.Size))

			if config.PrintBody && headerContentLength > 0 && headerContentLength < 1*bytes.KB {
				body, err := ioutil.ReadAll(req.Body)
				if err != nil {
					log.Warn("echozap error decoding request body", zap.Error(err))
				} else {
					fields = append(fields, zap.String("body", string(body)))
				}
			}

			if err != nil {
				fields = append(fields, zap.Error(err))
				c.Error(err)

				if he, ok := err.(*echo.HTTPError); ok {
					if he.Internal != nil {
						fields = append(fields, zap.NamedError("internal_error", he.Internal))
					}
				}
			}

			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
				fields = append(fields, zap.String("request_id", id))
			}

			n := res.Status
			switch {
			case n >= 500:
				log.With(zap.Error(err)).Error("Server error", fields...)
			case n >= 400:
				log.With(zap.Error(err)).Warn("Client error", fields...)
			case n >= 300:
				log.Info("Redirection", fields...)
			default:
				log.Info("Success", fields...)
			}

			return nil
		}
	}
}

// DefaultSkipper returns false which processes the middleware.
func DefaultSkipper(echo.Context) bool {
	return false
}

func extendWithCtx(ctx context.Context, log *zap.Logger, keys ...interface{}) {
	for _, key := range keys {
		v := ctx.Value(key)
		if v == nil {
			continue
		}

		log.With(zap.Any(fmt.Sprintf("%v", key), v))
	}
}
