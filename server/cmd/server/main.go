package main

import (
	"context"
	"log"
	"os"

	"mine-and-die/server/internal/app"
	"mine-and-die/server/internal/telemetry"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	if err := app.Run(context.Background(), app.Config{Logger: telemetry.WrapLogger(logger)}); err != nil {
		log.Fatalf("%v", err)
	}
}
