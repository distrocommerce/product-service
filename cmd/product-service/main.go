package main

import (
	"log/slog"
	"os"
)

// version is set at build time: -ldflags "-X main.version=..."
var version = "dev"

func main() {
	slog.New(slog.NewJSONHandler(os.Stdout, nil)).Info("starting", "service", "product", "version", version)
}
