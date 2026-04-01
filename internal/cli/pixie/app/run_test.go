package app

import "testing"

func TestNewRootCmdIncludesDBShell(t *testing.T) {
	root := NewRootCmd()

	found := false
	for _, command := range root.Commands() {
		if command.Name() == "db-shell" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("db-shell command not registered on root command")
	}
}
