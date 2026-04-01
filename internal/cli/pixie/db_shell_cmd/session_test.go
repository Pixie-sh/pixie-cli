package db_shell_cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellRunWithSQLiteSession(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shell.db")
	executor, err := OpenExecutor(context.Background(), ResolvedConfig{
		Driver: "sqlite",
		DSN:    "file:" + dbPath,
	})
	if err != nil {
		t.Fatalf("OpenExecutor() error = %v", err)
	}

	input := strings.NewReader(strings.Join([]string{
		"create table users (id integer primary key, name text);",
		"insert into users (name) values ('ada');",
		"bad sql;",
		"select name from users;",
		".exit",
	}, "\n"))

	var stdout strings.Builder
	var stderr strings.Builder

	shell := Shell{
		Executor: executor,
		In:       input,
		Out:      &stdout,
		ErrOut:   &stderr,
		Prompt:   "pixie-sql> ",
	}

	if err := shell.Run(context.Background()); err != nil {
		t.Fatalf("Shell.Run() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Connected to sqlite") {
		t.Fatalf("stdout missing sqlite connection banner: %s", output)
	}
	if !strings.Contains(output, "ada") {
		t.Fatalf("stdout missing query result: %s", output)
	}
	if !strings.Contains(output, "Session closed.") {
		t.Fatalf("stdout missing session close message: %s", output)
	}
	if !strings.Contains(stderr.String(), "SQL error:") {
		t.Fatalf("stderr missing SQL error output: %s", stderr.String())
	}
}

func TestShellHelpBuiltin(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "help.db")
	executor, err := OpenExecutor(context.Background(), ResolvedConfig{
		Driver: "sqlite",
		DSN:    "file:" + dbPath,
	})
	if err != nil {
		t.Fatalf("OpenExecutor() error = %v", err)
	}

	var stdout strings.Builder
	shell := Shell{
		Executor: executor,
		In:       strings.NewReader(".help\n.exit\n"),
		Out:      &stdout,
		ErrOut:   &stdout,
	}

	if err := shell.Run(context.Background()); err != nil {
		t.Fatalf("Shell.Run() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Built-ins:") {
		t.Fatalf("help output missing built-ins header: %s", output)
	}
	if !strings.Contains(output, ".exit") {
		t.Fatalf("help output missing .exit docs: %s", output)
	}
}
