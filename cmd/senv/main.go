package main

import (
	"os"

	"github.com/dnery/dotstate/dot/internal/senv"
)

func main() {
	os.Exit(senv.Execute(os.Args[1:]))
}
