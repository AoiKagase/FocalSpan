package main

import (
	"context"
	"github.com/focalspan/focalspan/internal/benchcli"
	"os"
)

func main() { os.Exit(benchcli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) }
