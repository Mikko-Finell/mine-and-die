package main

import (
	"log"
	"os"

	"mine-and-die/tools/effectsgen/internal/cli"
)

func main() {
	if err := cli.Execute(os.Stdout, os.Stderr); err != nil {
		log.Fatal(err)
	}
}
