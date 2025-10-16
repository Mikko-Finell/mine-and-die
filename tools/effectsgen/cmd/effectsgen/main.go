package main

import (
	"log"
	"os"

	"mine-and-die/tools/effectsgen/internal/cli"
)

func main() {
	if err := cli.Execute(os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
