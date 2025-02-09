package main

import (
	"context"

	"github.com/adnvilla/logger-go"
	"github.com/adnvilla/logger-go/zap"

	zaplib "go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	zapLogger, _ := zaplib.NewDevelopment()
	ctx = logger.SetLogger(ctx, zap.NewHandler(zapLogger))

	logger.Info(ctx, "Hello, World!")
	logger.Debug(ctx, "Hello, World!", "key", "value", "key2", 123)

	zapLogger.Log(zaplib.InfoLevel, "Hello, World!")
}
