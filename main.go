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
	"github.com/utgwkk/dynamodb-local-proxy/internal/handler"
)

func main() {
	ctx := context.Background()

	slog.SetDefault(
		slog.New(
			slogcontext.NewHandler(
				slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}),
			),
		),
	)
	h := handler.New(
		os.Getenv("DYNAMODB_LOCAL_ADDR"),
		http.DefaultClient,
	)

	if err := gracefulhttp.ListenAndServe(
		ctx,
		os.Getenv("DYNAMODB_LOCAL_PROXY_ADDR"),
		h,
		graceful.GracefulShutdownTimeout(10*time.Second),
	); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.ErrorContext(ctx, "failed to listen", slog.Any("error", err))
	}
}
