package main

import (
	"os"

	"github.com/dnery/dotstate/dot/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
