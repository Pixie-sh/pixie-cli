package db_shell_cmd

import (
	"bufio"
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"io"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pixie-sh/errors-go"
	_ "modernc.org/sqlite"
)

type ExecutionResult struct {
	Columns      []string
	Rows         [][]string
	RowsAffected int64
	IsQuery      bool
}

type Executor interface {
	Execute(context.Context, string) (ExecutionResult, error)
	Close() error
	Summary() string
}

type sqlExecutor struct {
	db      *sql.DB
	summary string
}

type Shell struct {
	Executor Executor
	In       io.Reader
	Out      io.Writer
	ErrOut   io.Writer
	Prompt   string
}

func OpenExecutor(ctx context.Context, cfg ResolvedConfig) (Executor, error) {
	db, err := sql.Open(cfg.SQLDriverName(), cfg.DSN)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open database connection")
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, errors.Wrap(err, "failed to connect to database")
	}

	return &sqlExecutor{db: db, summary: cfg.SafeSummary()}, nil
}

func (e *sqlExecutor) Summary() string {
	return e.summary
}

func (e *sqlExecutor) Close() error {
	return e.db.Close()
}

func (e *sqlExecutor) Execute(ctx context.Context, statement string) (ExecutionResult, error) {
	if isQueryStatement(statement) {
		return e.executeQuery(ctx, statement)
	}

	result, err := e.db.ExecContext(ctx, statement)
	if err != nil {
		return ExecutionResult{}, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}

	return ExecutionResult{RowsAffected: rowsAffected}, nil
}

func (e *sqlExecutor) executeQuery(ctx context.Context, statement string) (ExecutionResult, error) {
	rows, err := e.db.QueryContext(ctx, statement)
	if err != nil {
		return ExecutionResult{}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return ExecutionResult{}, err
	}

	formattedRows := make([][]string, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		scans := make([]any, len(columns))
		for index := range values {
			scans[index] = &values[index]
		}

		if err := rows.Scan(scans...); err != nil {
			return ExecutionResult{}, err
		}

		formattedRow := make([]string, len(columns))
		for index, value := range values {
			formattedRow[index] = stringifyValue(value)
		}
		formattedRows = append(formattedRows, formattedRow)
	}

	if err := rows.Err(); err != nil {
		return ExecutionResult{}, err
	}

	return ExecutionResult{Columns: columns, Rows: formattedRows, IsQuery: true}, nil
}

func (s Shell) Run(ctx context.Context) error {
	if s.Executor == nil {
		return errors.New("executor is required")
	}
	if s.In == nil {
		return errors.New("input reader is required")
	}
	if s.Out == nil {
		return errors.New("output writer is required")
	}
	if s.ErrOut == nil {
		s.ErrOut = s.Out
	}
	if s.Prompt == "" {
		s.Prompt = "pixie-sql> "
	}

	defer s.Executor.Close()

	fmt.Fprintf(s.Out, "Connected to %s\n", s.Executor.Summary())
	fmt.Fprintln(s.Out, "Enter SQL statements or .help for shell commands.")

	lines := make(chan lineResult, 1)
	go readLines(s.In, lines)

	for {
		fmt.Fprint(s.Out, s.Prompt)

		select {
		case <-ctx.Done():
			fmt.Fprintln(s.ErrOut, "Interrupted. Closing session.")
			return nil
		case result, ok := <-lines:
			if !ok || result.eof {
				fmt.Fprintln(s.Out)
				fmt.Fprintln(s.Out, "Session closed.")
				return nil
			}
			if result.err != nil {
				return errors.Wrap(result.err, "failed to read input")
			}

			statement := strings.TrimSpace(result.line)
			if statement == "" {
				continue
			}

			handled, shouldExit := handleBuiltin(s.Out, statement)
			if handled {
				if shouldExit {
					fmt.Fprintln(s.Out, "Session closed.")
					return nil
				}
				continue
			}

			executionResult, err := s.Executor.Execute(ctx, statement)
			if err != nil {
				fmt.Fprintf(s.ErrOut, "SQL error: %v\n", err)
				continue
			}

			writeResult(s.Out, executionResult)
		}
	}
}

type lineResult struct {
	line string
	err  error
	eof  bool
}

func readLines(input io.Reader, output chan<- lineResult) {
	defer close(output)

	reader := bufio.NewReader(input)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if stderrors.Is(err, io.EOF) {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					output <- lineResult{line: trimmed}
				}
				output <- lineResult{eof: true}
				return
			}
			output <- lineResult{err: err}
			return
		}

		output <- lineResult{line: strings.TrimRight(line, "\r\n")}
	}
}

func handleBuiltin(output io.Writer, statement string) (bool, bool) {
	switch statement {
	case ".help":
		fmt.Fprintln(output, "Built-ins:")
		fmt.Fprintln(output, "  .help  Show available shell commands")
		fmt.Fprintln(output, "  .exit  Close the shell session")
		fmt.Fprintln(output, "  .quit  Close the shell session")
		return true, false
	case ".exit", ".quit":
		return true, true
	default:
		if strings.HasPrefix(statement, ".") {
			fmt.Fprintf(output, "Unknown shell command: %s\n", statement)
			fmt.Fprintln(output, "Use .help to see supported commands.")
			return true, false
		}
	}

	return false, false
}

func writeResult(output io.Writer, result ExecutionResult) {
	if result.IsQuery {
		if len(result.Columns) > 0 {
			fmt.Fprintln(output, strings.Join(result.Columns, "\t"))
		}
		for _, row := range result.Rows {
			fmt.Fprintln(output, strings.Join(row, "\t"))
		}
		fmt.Fprintf(output, "%d row(s)\n", len(result.Rows))
		return
	}

	fmt.Fprintf(output, "OK (%d row(s) affected)\n", result.RowsAffected)
}

func isQueryStatement(statement string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(statement))
	for _, prefix := range []string{"select", "pragma", "with", "show", "describe", "explain"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	return false
}

func stringifyValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}
