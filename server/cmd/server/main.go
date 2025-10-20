package main

import (
	"context"
	"log"

	"mine-and-die/server/internal/app"
)

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatalf("%v", err)
	}
}
