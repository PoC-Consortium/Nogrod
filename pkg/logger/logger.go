// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package logger

import (
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
	"net/http"
)

var Logger *zap.Logger

func init() {
	// a fallback/root logger for events without context
	Logger, _ = zap.NewProduction()
	defer Logger.Sync()
}

func RequestLogger(req *http.Request) *zap.Logger {
	if req.Header.Get("X-Request-ID") == "" {
		req.Header.Set("X-Request-ID", uuid.Must(uuid.NewV4(), nil).String())
	}
	return Logger.With(zap.String("requestId", req.Header.Get("X-Request-ID")))
}
