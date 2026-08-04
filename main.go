package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/build"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/config"
	"github.com/bovinemagnet/graphqls-to-asciidoc/pkg/daemon"
)

var (
	Version   = "development"
	BuildTime = "unknown"
)

func init() {
	// Set version variables in config package
	config.Version = Version
	config.BuildTime = BuildTime
}

func main() {
	cfg := config.ParseFlags()

	if cfg.HandleVersion() {
		os.Exit(0)
	}

	if cfg.HandleHelp() {
		os.Exit(0)
	}

	if err := cfg.Validate(); err != nil {
		config.PrintError(err.Error())
		os.Exit(1)
	}

	if cfg.Daemon {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		err := daemon.Run(ctx, cfg)
		stop()
		if err != nil {
			log.Fatalf("daemon failed: %v", err)
		}
		return
	}

	result, err := build.Run(cfg)
	if err != nil {
		log.Fatalf("%v", err)
	}

	outputWriter, shouldClose, err := cfg.GetOutputWriter()
	if err != nil {
		log.Fatalf("Failed to setup output: %v", err)
	}

	if _, err := outputWriter.Write(result.Content); err != nil {
		log.Fatalf("Failed to write output: %v", err)
	}

	if shouldClose {
		if closeErr := outputWriter.Close(); closeErr != nil {
			log.Fatalf("Failed to close output file: %v", closeErr)
		}
	}
}
