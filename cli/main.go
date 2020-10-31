package main

import (
	"fmt"
	"os"

	"github.com/KosukeOhmura/room_crawler/src"
)

const (
	exitCodeOK  = 0
	exitCodeErr = 1
)

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if err := src.Execute(); err != nil {
		if notifyErr := src.NotifyError(err); notifyErr != nil {
			fmt.Printf("failed to notify err. notify err: %s, err: %s\n", notifyErr, err)
		}
		return exitCodeErr
	}
	return exitCodeOK
}
