//go:build windows

package db_shell_cmd

import "os"

var interruptSignals = []os.Signal{os.Interrupt}
