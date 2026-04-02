package db_shell_cmd

import (
	"bufio"
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	_ "github.com/jackc/pgx/v5/stdlib"
	helperdb "github.com/pixie-sh/database-helpers-go/database"
	"github.com/pixie-sh/errors-go"
	"golang.org/x/term"
	_ "modernc.org/sqlite"
)

const (
	defaultRenderWidth   = 100
	maxTableColumnWidth  = 32
	interactiveHistorySz = 500
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

type helperConnection interface {
	Ping() error
	Close() error
	SQLDB() (*sql.DB, error)
}

type helperConnectionAdapter struct {
	orm *helperdb.Orm
}

var (
	openSQLiteExecutorFunc = openSQLiteExecutor
	openHelperExecutorFunc = openHelperExecutor
	openHelperConnection   = func(ctx context.Context, cfg *helperdb.Configuration) (helperConnection, error) {
		orm, err := helperdb.FactoryInstance.Create(ctx, cfg)
		if err != nil {
			return nil, err
		}

		return helperConnectionAdapter{orm: orm}, nil
	}
	newReadlineInstance = func(cfg *readline.Config) (interactiveConsole, error) {
		return readline.NewEx(cfg)
	}
	isTerminalFunc = func(file *os.File) bool {
		return term.IsTerminal(int(file.Fd()))
	}
	terminalSizeFunc = func(file *os.File) (int, int, error) {
		return term.GetSize(int(file.Fd()))
	}
	newLineReaderFunc = newLineReader
)

type Shell struct {
	Executor Executor
	In       io.Reader
	Out      io.Writer
	ErrOut   io.Writer
	Prompt   string
}

type lineReader interface {
	ReadLine(context.Context) lineResult
	AddHistory(string)
	Close() error
}

type interactiveConsole interface {
	Readline() (string, error)
	SaveHistory(string) error
	Close() error
}

type bufferedLineReader struct {
	reader *bufio.Reader
	output io.Writer
	prompt string
}

type readlineLineReader struct {
	console interactiveConsole
}

func OpenExecutor(ctx context.Context, cfg ResolvedConfig) (Executor, error) {
	if cfg.IsSQLite() {
		return openSQLiteExecutorFunc(ctx, cfg)
	}

	return openHelperExecutorFunc(ctx, cfg)
}

func openSQLiteExecutor(ctx context.Context, cfg ResolvedConfig) (Executor, error) {
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

func openHelperExecutor(ctx context.Context, cfg ResolvedConfig) (Executor, error) {
	conn, err := openHelperConnection(ctx, buildHelperConfiguration(cfg))
	if err != nil {
		return nil, errors.Wrap(err, "failed to open helper-backed database connection")
	}

	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, errors.Wrap(err, "failed to connect to database")
	}

	rawDB, err := conn.SQLDB()
	if err != nil {
		_ = conn.Close()
		return nil, errors.Wrap(err, "failed to access raw database handle")
	}

	return &sqlExecutor{db: rawDB, summary: cfg.SafeSummary()}, nil
}

func (a helperConnectionAdapter) Ping() error {
	return a.orm.Ping()
}

func (a helperConnectionAdapter) Close() error {
	return a.orm.Close()
}

func (a helperConnectionAdapter) SQLDB() (*sql.DB, error) {
	return a.orm.DB.DB()
}

func buildHelperConfiguration(cfg ResolvedConfig) *helperdb.Configuration {
	return &helperdb.Configuration{
		Driver: helperdb.GormDriver,
		Values: &helperdb.GormDbConfiguration{
			Driver: helperdb.PsqlDriver,
			Dsn:    cfg.DSN,
		},
	}
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

	reader, err := newLineReaderFunc(s.In, s.Out, s.ErrOut, s.Prompt)
	if err != nil {
		return errors.Wrap(err, "failed to initialize shell input")
	}
	defer reader.Close()

	for {
		if ctx.Err() != nil {
			fmt.Fprintln(s.ErrOut, "Interrupted. Closing session.")
			return nil
		}

		result := reader.ReadLine(ctx)
		if result.interrupted {
			fmt.Fprintln(s.ErrOut, "Interrupted. Closing session.")
			return nil
		}
		if result.eof {
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

		reader.AddHistory(statement)

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

type lineResult struct {
	line        string
	err         error
	eof         bool
	interrupted bool
}

func newLineReader(input io.Reader, output io.Writer, errOutput io.Writer, prompt string) (lineReader, error) {
	inputFile, outputFile, interactive := terminalFiles(input, output)
	if !interactive {
		return &bufferedLineReader{
			reader: bufio.NewReader(input),
			output: output,
			prompt: prompt,
		}, nil
	}

	console, err := newReadlineInstance(&readline.Config{
		Prompt:                 prompt,
		Stdin:                  inputFile,
		Stdout:                 outputFile,
		Stderr:                 errOutput,
		HistoryLimit:           interactiveHistorySz,
		DisableAutoSaveHistory: true,
	})
	if err != nil {
		fmt.Fprintf(errOutput, "Warning: interactive line editing unavailable, falling back to basic input: %v\n", err)
		return &bufferedLineReader{
			reader: bufio.NewReader(input),
			output: output,
			prompt: prompt,
		}, nil
	}

	return &readlineLineReader{console: console}, nil
}

func terminalFiles(input io.Reader, output io.Writer) (*os.File, *os.File, bool) {
	inputFile, inputOK := input.(*os.File)
	outputFile, outputOK := output.(*os.File)
	if !inputOK || !outputOK {
		return nil, nil, false
	}
	if !isTerminalFunc(inputFile) || !isTerminalFunc(outputFile) {
		return nil, nil, false
	}

	return inputFile, outputFile, true

}

func (r *bufferedLineReader) ReadLine(ctx context.Context) lineResult {
	fmt.Fprint(r.output, r.prompt)

	resultChan := make(chan lineResult, 1)
	go func() {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			if stderrors.Is(err, io.EOF) {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					resultChan <- lineResult{line: trimmed}
					return
				}
				resultChan <- lineResult{eof: true}
				return
			}
			resultChan <- lineResult{err: err}
			return
		}

		resultChan <- lineResult{line: strings.TrimRight(line, "\r\n")}
	}()

	select {
	case <-ctx.Done():
		return lineResult{interrupted: true}
	case result := <-resultChan:
		return result
	}
}

func (r *bufferedLineReader) AddHistory(string) {}

func (r *bufferedLineReader) Close() error { return nil }

func (r *readlineLineReader) ReadLine(context.Context) lineResult {
	line, err := r.console.Readline()
	if err != nil {
		if stderrors.Is(err, readline.ErrInterrupt) {
			return lineResult{interrupted: true}
		}
		if stderrors.Is(err, io.EOF) {
			return lineResult{eof: true}
		}
		return lineResult{err: err}
	}

	return lineResult{line: strings.TrimRight(line, "\r\n")}
}

func (r *readlineLineReader) AddHistory(line string) {
	_ = r.console.SaveHistory(line)
}

func (r *readlineLineReader) Close() error {
	return r.console.Close()
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
		writeQueryResult(output, result, outputWidth(output))
		fmt.Fprintf(output, "%d row(s)\n", len(result.Rows))
		return
	}

	fmt.Fprintf(output, "OK (%d row(s) affected)\n", result.RowsAffected)
}

func writeQueryResult(output io.Writer, result ExecutionResult, width int) {
	if len(result.Columns) == 0 {
		return
	}
	if shouldUseVerticalLayout(result, width) {
		writeVerticalResult(output, result, width)
		return
	}
	writeTableResult(output, result)
}

func shouldUseVerticalLayout(result ExecutionResult, width int) bool {
	columnWidths := calculateColumnWidths(result)
	totalWidth := 1
	for _, columnWidth := range columnWidths {
		totalWidth += columnWidth + 3
	}

	return totalWidth > normalizeRenderWidth(width)
}

func writeTableResult(output io.Writer, result ExecutionResult) {
	columnWidths := calculateColumnWidths(result)
	separator := buildTableSeparator(columnWidths)

	fmt.Fprintln(output, separator)
	writeTableRow(output, result.Columns, columnWidths)
	fmt.Fprintln(output, separator)
	for _, row := range result.Rows {
		writeTableRow(output, row, columnWidths)
	}
	fmt.Fprintln(output, separator)
}

func writeTableRow(output io.Writer, row []string, widths []int) {
	formatted := make([]string, len(widths))
	for index, width := range widths {
		value := ""
		if index < len(row) {
			value = normalizeCell(row[index])
		}
		formatted[index] = padRight(truncateForWidth(value, width), width)
	}

	fmt.Fprintf(output, "| %s |\n", strings.Join(formatted, " | "))
}

func buildTableSeparator(widths []int) string {
	segments := make([]string, len(widths))
	for index, width := range widths {
		segments[index] = strings.Repeat("-", width+2)
	}

	return "+" + strings.Join(segments, "+") + "+"
}

func writeVerticalResult(output io.Writer, result ExecutionResult, width int) {
	labelWidth := 0
	for _, column := range result.Columns {
		labelWidth = max(labelWidth, len(column))
	}
	available := max(20, normalizeRenderWidth(width)-labelWidth-6)

	for rowIndex, row := range result.Rows {
		header := fmt.Sprintf("-[ RECORD %d ]", rowIndex+1)
		lineWidth := max(len(header)+1, normalizeRenderWidth(width))
		fmt.Fprintf(output, "%s%s\n", header, strings.Repeat("-", lineWidth-len(header)))
		for columnIndex, column := range result.Columns {
			value := ""
			if columnIndex < len(row) {
				value = row[columnIndex]
			}
			fmt.Fprintf(output, "%s | %s\n", padRight(column, labelWidth), truncateForWidth(normalizeCell(value), available))
		}
	}
	if len(result.Rows) == 0 {
		fmt.Fprintln(output, "(no rows)")
	}
}

func calculateColumnWidths(result ExecutionResult) []int {
	widths := make([]int, len(result.Columns))
	for index, column := range result.Columns {
		widths[index] = min(maxTableColumnWidth, max(1, len(normalizeCell(column))))
	}
	for _, row := range result.Rows {
		for index := range result.Columns {
			if index >= len(row) {
				continue
			}
			widths[index] = min(maxTableColumnWidth, max(widths[index], len(normalizeCell(row[index]))))
		}
	}

	return widths
}

func outputWidth(output io.Writer) int {
	file, ok := output.(*os.File)
	if !ok || !isTerminalFunc(file) {
		return defaultRenderWidth
	}

	width, _, err := terminalSizeFunc(file)
	if err != nil || width <= 0 {
		return defaultRenderWidth
	}

	return width
}

func normalizeRenderWidth(width int) int {
	if width <= 0 {
		return defaultRenderWidth
	}

	return width
}

func normalizeCell(value string) string {
	replacer := strings.NewReplacer("\r\n", "\\n", "\n", "\\n", "\r", "\\r", "\t", " ")
	return replacer.Replace(value)
}

func truncateForWidth(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}

	return value[:width-3] + "..."
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}

	return value + strings.Repeat(" ", width-len(value))
}

func min(left, right int) int {
	if left < right {
		return left
	}

	return right
}

func max(left, right int) int {
	if left > right {
		return left
	}

	return right
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
