package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/KarlGW/go-template/templates/http-server/server"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v", err)
	}
}

func run(ctx context.Context) error {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	srv := server.New(server.WithOptions(server.Options{
		Logger: log,
	}))

	if err := srv.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Server error.", "error", err)
		return err
	}
	return nil
}
