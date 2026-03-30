package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	slogcontext "github.com/PumpkinSeed/slog-context"
	"github.com/ne-sachirou/go-graceful"
	"github.com/ne-sachirou/go-graceful/gracefulhttp"
	"github.com/utgwkk/dynamodb-local-proxy/internal/config"
	"github.com/utgwkk/dynamodb-local-proxy/internal/handler"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Parse()
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse environment variables", slog.Any("error", err))
		os.Exit(1)
	}

	slog.SetDefault(
		slog.New(
			slogcontext.NewHandler(
				slog.NewJSONHandler(
					os.Stdout,
					&slog.HandlerOptions{Level: cfg.LogLevel},
				),
			),
		),
	)

	h := handler.New(
		cfg.DynamoDBLocalAddr,
		http.DefaultClient,
	)

	slog.InfoContext(ctx, "listening", slog.String("addr", cfg.BindAddr()))

	if err := gracefulhttp.ListenAndServe(
		ctx,
		cfg.BindAddr(),
		h,
		graceful.GracefulShutdownTimeout(10*time.Second),
	); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.ErrorContext(ctx, "failed to listen", slog.Any("error", err))
	}
}
