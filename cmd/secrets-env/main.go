package main

import (
	"os"

	"github.com/dnery/dotstate/dot/internal/secretsenv"
)

func main() {
	os.Exit(secretsenv.Execute(os.Args[1:]))
}
