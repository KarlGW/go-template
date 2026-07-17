package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/KarlGW/go-template/templates/http-service/internal/server"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	srv := server.New(
		server.WithLogger(log),
	)

	if err := srv.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
