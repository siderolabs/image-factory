// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main is the entrypoint of the image factory.
package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"

	"github.com/siderolabs/go-debug"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func runDebugServer(ctx context.Context) {
	const debugAddr = ":9981"

	debugLogFunc := func(msg string) {
		log.Print(msg)
	}

	if err := debug.ListenAndServe(ctx, debugAddr, debugLogFunc); err != nil {
		log.Fatalf("failed to start debug server: %s", err)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM)
	defer cancel()

	go runDebugServer(ctx)

	return runWithContext(ctx)
}

func runWithContext(ctx context.Context) error {
	opts := initFlags()

	if opts.ExtensionNameAlias != "" {
		if err := config.LoadAliases(opts.ExtensionNameAlias); err != nil {
			log.Fatalf("failed to load extension aliases: %v", err)
		}
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(*opts.LogLevel)

	logger, err := cfg.Build()
	if err != nil {
		return fmt.Errorf("failed to initialize production logger: %w", err)
	}

	logger.Debug("starting factory", zap.Any("level", *opts.LogLevel))

	return cmd.RunFactory(ctx, logger, opts)
}
