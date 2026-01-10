package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/KarlGW/go-template/templates/service/internal/service"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running service: %v", err)
	}
}

func run(_ context.Context) error {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	svc := service.New(
		service.WithLogger(log),
	)

	if err := svc.Start(); err != nil {
		log.Error("Service error.", "error", err)
		return err
	}
	return nil
}
