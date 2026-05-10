package main

import (
	"fmt"
	"os"

	"github.com/thedavidweng/monarchmoney-cli/internal/cli"
)

func main() {
	if err := cli.New(cli.DefaultDeps()).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
