package zap_test

import (
	"context"
	"log/slog"
	"os"

	logger "github.com/adnvilla/logger-go"
	loggerzap "github.com/adnvilla/logger-go/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type password string

func (password) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

func ExampleNewHandler() {
	// Any zap.Logger works, for example zap.NewProduction(). This one writes
	// JSON to stdout without timestamps so the output below is stable.
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	})
	zapLogger := zap.New(zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel))

	ctx := logger.SetLogger(context.Background(), loggerzap.NewHandler(zapLogger))
	ctx = logger.With(ctx, "service", "checkout")

	logger.Info(ctx, "login", slog.Group("user", "id", "u1", "password", password("hunter2")))
	// Output:
	// {"level":"info","msg":"login","service":"checkout","user":{"id":"u1","password":"[REDACTED]"}}
}
