package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/KarlGW/go-template/templates/service/internal/server"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running server: %v", err)
	}
}

func run(ctx context.Context) error {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	srv := server.New(
		server.WithLogger(log),
	)

	if err := srv.Start(ctx); err != nil {
		log.Error("Server error.", "error", err)
		return err
	}
	return nil
}
