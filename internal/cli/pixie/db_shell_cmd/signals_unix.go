//go:build !windows

package db_shell_cmd

import (
	"os"
	"syscall"
)

var interruptSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
