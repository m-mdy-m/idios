package main

import (
	"errors"
	"fmt"
	"os"

	"idios/internal/tuilaunch"
)

func main() {
	if err := tuilaunch.Run(false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, tuilaunch.ErrNonInteractive) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
