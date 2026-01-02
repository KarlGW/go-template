package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	server "github.com/KarlGW/go-template/templates/server/server"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v", err)
	}
}

func run(_ context.Context) error {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	srv := server.New(
		server.WithLogger(log),
	)

	if err := srv.Start(); err != nil {
		log.Error("Server error.", "error", err)
		return err
	}
	return nil
}
