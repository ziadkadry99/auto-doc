package main

import (
	"os"

	"github.com/ziadkadry99/auto-doc/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
