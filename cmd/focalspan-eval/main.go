package main

import (
	"context"
	"os"

	"github.com/focalspan/focalspan/internal/evalcli"
)

func main() {
	os.Exit(evalcli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
