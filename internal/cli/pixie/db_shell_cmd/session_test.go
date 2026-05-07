package db_shell_cmd

import (
	"context"
	"database/sql"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chzyer/readline"
	helperdb "github.com/pixie-sh/database-helpers-go/database"
)

type stubExecutor struct {
	summary string
}

type scriptedExecutor struct {
	results map[string]ExecutionResult
	errs    map[string]error
}

type fakeInteractiveConsole struct {
	lines   []string
	errs    []error
	index   int
	history []string
	closed  bool
}

func (s stubExecutor) Execute(context.Context, string) (ExecutionResult, error) {
	return ExecutionResult{}, nil
}

func (s stubExecutor) Close() error {
	return nil
}

func (s stubExecutor) Summary() string {
	return s.summary
}

func (s scriptedExecutor) Execute(_ context.Context, statement string) (ExecutionResult, error) {
	if err, ok := s.errs[statement]; ok {
		return ExecutionResult{}, err
	}
	if result, ok := s.results[statement]; ok {
		return result, nil
	}
	return ExecutionResult{}, nil
}

func (s scriptedExecutor) Close() error {
	return nil
}

func (s scriptedExecutor) Summary() string {
	return "scripted"
}

func (f *fakeInteractiveConsole) Readline() (string, error) {
	if f.index < len(f.errs) && f.errs[f.index] != nil {
		err := f.errs[f.index]
		f.index++
		return "", err
	}
	if f.index >= len(f.lines) {
		return "", io.EOF
	}
	line := f.lines[f.index]
	f.index++
	return line, nil
}

type fakeLineReader struct {
	results []lineResult
	index   int
	history []string
	closed  bool
}

func (f *fakeLineReader) ReadLine(context.Context) lineResult {
	if f.index >= len(f.results) {
		return lineResult{eof: true}
	}
	result := f.results[f.index]
	f.index++
	return result
}

func (f *fakeLineReader) AddHistory(line string) {
	f.history = append(f.history, line)
}

func (f *fakeLineReader) Close() error {
	f.closed = true
	return nil
}

func (f *fakeInteractiveConsole) SaveHistory(line string) error {
	f.history = append(f.history, line)
	return nil
}

func (f *fakeInteractiveConsole) Close() error {
	f.closed = true
	return nil
}

type fakeHelperConnection struct {
	db      *sql.DB
	pingErr error
	closeFn func() error
}

func (f fakeHelperConnection) Ping() error {
	return f.pingErr
}

func (f fakeHelperConnection) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	if f.db != nil {
		return f.db.Close()
	}
	return nil
}

func (f fakeHelperConnection) SQLDB() (*sql.DB, error) {
	if f.db == nil {
		return nil, stderrors.New("missing db")
	}
	return f.db, nil
}

func restoreExecutorOpeners() {
	openSQLiteExecutorFunc = openSQLiteExecutor
	openHelperExecutorFunc = openHelperExecutor
	openHelperConnection = func(ctx context.Context, cfg *helperdb.Configuration) (helperConnection, error) {
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
		return false
	}
	terminalSizeFunc = func(file *os.File) (int, int, error) {
		return 0, 0, stderrors.New("not a terminal")
	}
	newLineReaderFunc = newLineReader
}

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

func TestOpenExecutorRoutesSQLiteToSQLiteOpener(t *testing.T) {
	defer restoreExecutorOpeners()

	openHelperExecutorFunc = func(context.Context, ResolvedConfig) (Executor, error) {
		t.Fatal("helper opener should not be used for sqlite")
		return nil, nil
	}
	openSQLiteExecutorFunc = func(_ context.Context, cfg ResolvedConfig) (Executor, error) {
		if cfg.Driver != "sqlite" {
			t.Fatalf("sqlite opener driver = %q, want sqlite", cfg.Driver)
		}
		return stubExecutor{summary: "sqlite-route"}, nil
	}

	executor, err := OpenExecutor(context.Background(), ResolvedConfig{Driver: "sqlite", DSN: defaultSQLiteDSN})
	if err != nil {
		t.Fatalf("OpenExecutor() error = %v", err)
	}
	if summary := executor.Summary(); summary != "sqlite-route" {
		t.Fatalf("Summary() = %q, want sqlite-route", summary)
	}
}

func TestOpenExecutorRoutesPostgresToHelperOpener(t *testing.T) {
	defer restoreExecutorOpeners()

	openSQLiteExecutorFunc = func(context.Context, ResolvedConfig) (Executor, error) {
		t.Fatal("sqlite opener should not be used for postgres")
		return nil, nil
	}
	openHelperExecutorFunc = func(_ context.Context, cfg ResolvedConfig) (Executor, error) {
		if cfg.Driver != "postgres" {
			t.Fatalf("helper opener driver = %q, want postgres", cfg.Driver)
		}
		return stubExecutor{summary: "helper-route"}, nil
	}

	executor, err := OpenExecutor(context.Background(), ResolvedConfig{Driver: "postgres"})
	if err != nil {
		t.Fatalf("OpenExecutor() error = %v", err)
	}
	if summary := executor.Summary(); summary != "helper-route" {
		t.Fatalf("Summary() = %q, want helper-route", summary)
	}
}

func TestOpenHelperExecutorBootstrapFailure(t *testing.T) {
	defer restoreExecutorOpeners()

	openHelperConnection = func(context.Context, *helperdb.Configuration) (helperConnection, error) {
		return nil, stderrors.New("factory boom")
	}

	_, err := openHelperExecutor(context.Background(), ResolvedConfig{Driver: "postgres", DSN: "postgres://pixie"})
	if err == nil {
		t.Fatal("openHelperExecutor() error = nil, want bootstrap failure")
	}
	if !strings.Contains(err.Error(), "failed to open helper-backed database connection") {
		t.Fatalf("error = %q, want wrapped helper bootstrap failure", err.Error())
	}
}

func TestOpenHelperExecutorPingFailureClosesConnection(t *testing.T) {
	defer restoreExecutorOpeners()

	rawDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	closed := false
	openHelperConnection = func(context.Context, *helperdb.Configuration) (helperConnection, error) {
		return fakeHelperConnection{
			db:      rawDB,
			pingErr: stderrors.New("ping boom"),
			closeFn: func() error {
				closed = true
				return rawDB.Close()
			},
		}, nil
	}

	_, err = openHelperExecutor(context.Background(), ResolvedConfig{Driver: "postgres", DSN: "postgres://pixie"})
	if err == nil {
		t.Fatal("openHelperExecutor() error = nil, want ping failure")
	}
	if !strings.Contains(err.Error(), "failed to connect to database") {
		t.Fatalf("error = %q, want ping failure wrapper", err.Error())
	}
	if !closed {
		t.Fatal("connection was not closed after ping failure")
	}
}

func TestOpenHelperExecutorSuccessUsesHelperDatabase(t *testing.T) {
	defer restoreExecutorOpeners()

	rawDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = rawDB.Close()
	})

	var received *helperdb.Configuration
	openHelperConnection = func(_ context.Context, cfg *helperdb.Configuration) (helperConnection, error) {
		received = cfg
		return fakeHelperConnection{db: rawDB}, nil
	}

	executor, err := openHelperExecutor(context.Background(), ResolvedConfig{
		Driver:  "postgres",
		DSN:     "host=localhost port=5432 dbname=pixie user=postgres sslmode=disable",
		Host:    "localhost",
		Port:    5432,
		Name:    "pixie",
		User:    "postgres",
		SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("openHelperExecutor() error = %v", err)
	}
	t.Cleanup(func() {
		_ = executor.Close()
	})

	if received == nil {
		t.Fatal("helper configuration was not forwarded")
	}
	if received.Driver != helperdb.GormDriver {
		t.Fatalf("helper driver = %q, want %q", received.Driver, helperdb.GormDriver)
	}

	if _, err := executor.Execute(context.Background(), "create table users (id integer primary key, name text);"); err != nil {
		t.Fatalf("create table via helper-backed executor error = %v", err)
	}
	if _, err := executor.Execute(context.Background(), "insert into users (name) values ('ada');"); err != nil {
		t.Fatalf("insert via helper-backed executor error = %v", err)
	}

	result, err := executor.Execute(context.Background(), "select name from users;")
	if err != nil {
		t.Fatalf("query via helper-backed executor error = %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][0] != "ada" {
		t.Fatalf("query result = %#v, want row with ada", result.Rows)
	}
}

func TestBuildHelperConfigurationUsesDriverAndDSN(t *testing.T) {
	cfg := buildHelperConfiguration(ResolvedConfig{
		Driver: "postgres",
		DSN:    "host=localhost port=5432 dbname=pixie user=postgres sslmode=disable",
	})

	if cfg.Driver != helperdb.GormDriver {
		t.Fatalf("Driver = %q, want %q", cfg.Driver, helperdb.GormDriver)
	}

	values, ok := cfg.Values.(*helperdb.GormDbConfiguration)
	if !ok {
		t.Fatalf("Values type = %T, want *database.GormDbConfiguration", cfg.Values)
	}

	if values.Driver != helperdb.PsqlDriver {
		t.Fatalf("Values.Driver = %q, want %q", values.Driver, helperdb.PsqlDriver)
	}

	if values.Dsn != "host=localhost port=5432 dbname=pixie user=postgres sslmode=disable" {
		t.Fatalf("Values.Dsn = %q, want source DSN", values.Dsn)
	}
}

func TestWriteResultUsesTableLayoutForNarrowResults(t *testing.T) {
	var output strings.Builder

	writeResult(&output, ExecutionResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "Ada"}},
		IsQuery: true,
	})

	rendered := output.String()
	if !strings.Contains(rendered, "+----+------+") {
		t.Fatalf("table output missing separator: %s", rendered)
	}
	if !strings.Contains(rendered, "| id | name |") {
		t.Fatalf("table output missing header row: %s", rendered)
	}
	if !strings.Contains(rendered, "| 1  | Ada  |") {
		t.Fatalf("table output missing data row: %s", rendered)
	}
}

func TestWriteQueryResultUsesVerticalLayoutWhenWide(t *testing.T) {
	var output strings.Builder

	writeQueryResult(&output, ExecutionResult{
		Columns: []string{"id", "description", "metadata"},
		Rows:    [][]string{{"1", strings.Repeat("x", 48), strings.Repeat("y", 48)}},
		IsQuery: true,
	}, 40)

	rendered := output.String()
	if !strings.Contains(rendered, "-[ RECORD 1 ]") {
		t.Fatalf("vertical output missing record header: %s", rendered)
	}
	if !strings.Contains(rendered, "description | ") {
		t.Fatalf("vertical output missing label/value row: %s", rendered)
	}
	if strings.Contains(rendered, "\t") {
		t.Fatalf("vertical output should not contain tab-separated rendering: %s", rendered)
	}
}

func TestNewLineReaderUsesBufferedReaderWhenNotTTY(t *testing.T) {
	defer restoreExecutorOpeners()

	reader, err := newLineReader(strings.NewReader("select 1;\n"), io.Discard, io.Discard, "pixie-sql> ")
	if err != nil {
		t.Fatalf("newLineReader() error = %v", err)
	}
	defer reader.Close()

	if _, ok := reader.(*bufferedLineReader); !ok {
		t.Fatalf("reader type = %T, want *bufferedLineReader", reader)
	}
}

func TestNewLineReaderUsesReadlineForInteractiveTTY(t *testing.T) {
	defer restoreExecutorOpeners()

	console := &fakeInteractiveConsole{}
	newReadlineInstance = func(cfg *readline.Config) (interactiveConsole, error) {
		if cfg.Prompt != "pixie-sql> " {
			t.Fatalf("prompt = %q, want pixie-sql> ", cfg.Prompt)
		}
		if cfg.HistoryLimit != interactiveHistorySz {
			t.Fatalf("history limit = %d, want %d", cfg.HistoryLimit, interactiveHistorySz)
		}
		if !cfg.DisableAutoSaveHistory {
			t.Fatal("expected history autosave to remain disabled")
		}
		return console, nil
	}
	isTerminalFunc = func(file *os.File) bool { return true }

	stdin, err := os.Open("/dev/null")
	if err != nil {
		t.Fatalf("os.Open(/dev/null) error = %v", err)
	}
	defer stdin.Close()
	stdout, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("os.OpenFile(/dev/null) error = %v", err)
	}
	defer stdout.Close()

	reader, err := newLineReader(stdin, stdout, io.Discard, "pixie-sql> ")
	if err != nil {
		t.Fatalf("newLineReader() error = %v", err)
	}
	defer reader.Close()

	interactive, ok := reader.(*readlineLineReader)
	if !ok {
		t.Fatalf("reader type = %T, want *readlineLineReader", reader)
	}
	interactive.AddHistory("select 1;")
	if len(console.history) != 1 || console.history[0] != "select 1;" {
		t.Fatalf("history = %#v, want saved statement", console.history)
	}
}

func TestShellRunAddsInteractiveHistoryAndExecutesStatements(t *testing.T) {
	defer restoreExecutorOpeners()

	reader := &fakeLineReader{results: []lineResult{{line: "select 1;"}, {line: ".exit"}}}
	newLineReaderFunc = func(io.Reader, io.Writer, io.Writer, string) (lineReader, error) {
		return reader, nil
	}

	var rendered strings.Builder
	shell := Shell{
		Executor: scriptedExecutor{results: map[string]ExecutionResult{
			"select 1;": {Columns: []string{"value"}, Rows: [][]string{{"1"}}, IsQuery: true},
		}},
		In:     strings.NewReader(""),
		Out:    &rendered,
		ErrOut: &rendered,
		Prompt: "pixie-sql> ",
	}

	if err := shell.Run(context.Background()); err != nil {
		t.Fatalf("Shell.Run() error = %v", err)
	}
	if len(reader.history) != 2 || reader.history[0] != "select 1;" || reader.history[1] != ".exit" {
		t.Fatalf("history = %#v, want both entered commands", reader.history)
	}
	if !reader.closed {
		t.Fatal("line reader was not closed")
	}
	if !strings.Contains(rendered.String(), "| value |") {
		t.Fatalf("rendered output missing query table: %s", rendered.String())
	}
}
