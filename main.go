package main

import (
	"context"
	"fmt"
	"os"

	"genealogymapper/internal/config"
	"genealogymapper/internal/genealogy"
	"genealogymapper/internal/graph"
	"genealogymapper/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	settings := config.Load()
	if err := settings.Validate(); err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	ctx := context.Background()
	repository, err := graph.NewNeo4jRepository(ctx, settings.Neo4j)
	if err != nil {
		return err
	}
	defer repository.Close(ctx)

	service := genealogy.NewService(repository)
	if err := ui.Run(service); err != nil {
		return fmt.Errorf("run genealogy mapper: %w", err)
	}
	return nil
}
