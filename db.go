package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/canonical/go-dqlite/client"
	"github.com/canonical/go-dqlite/driver"
	"github.com/pkg/errors"
)

var (
	dbxMu        sync.Mutex
	dbxDB        = make(map[string]*sql.DB)
	dbxDisableMu sync.Mutex // circuit breaker switch
	dbxDisabled  bool       // circuit breaker switch

	// ErrDatabaseUnavailable is returned if the database is unavailable
	ErrDatabaseUnavailable = errors.New("database is unavailable")
)

// EnableDatabase controls the database circuit breaker
func EnableDatabase(enable bool) {
	dbxDisableMu.Lock()
	defer dbxDisableMu.Unlock()
	dbxDisabled = !enable
}

// DatabaseDisabled indicates circuit breaker is set
func DatabaseDisabled() bool {
	dbxDisableMu.Lock()
	defer dbxDisableMu.Unlock()
	return dbxDisabled
}

type QDB interface {
	Header([]string)
	Body([]interface{})
	Close()
}

type DBTW struct {
	tw    *tabwriter.Writer
	multi bool
}

func (d *DBTW) Header(columns []string) {
	if d.multi {
		d.tw.Flush()
	}
	d.multi = true
	for i, column := range columns {
		if i > 0 {
			fmt.Fprint(d.tw, "\t")
		}
		fmt.Fprint(d.tw, column)
	}
	fmt.Fprintln(d.tw)
	for i, column := range columns {
		if i > 0 {
			fmt.Fprint(d.tw, "\t")
		}
		fmt.Fprint(d.tw, strings.Repeat("-", len(column)))
	}
}

func (d *DBTW) Body(buffer []interface{}) {
	for i, column := range buffer {
		if i > 0 {
			fmt.Fprint(d.tw, "\t")
		}
		fmt.Fprint(d.tw, column)
	}
	fmt.Fprintln(d.tw)
}

func (d *DBTW) Close() {
	d.tw.Flush()
}

var _ QDB = (*DBTW)(nil)

func (dx *DBX) queryer(qdb QDB, statement string, args ...interface{}) error {
	if DatabaseDisabled() {
		return ErrDatabaseUnavailable
	}
	rows, err := dx.db.Query(statement)
	if err != nil {
		return errors.Wrap(err, "dbx query failed")
	}

	defer qdb.Close()
	for {
		columns, err := rows.Columns()
		if err != nil {
			return err
		}

		qdb.Header(columns)

		buffer := make([]interface{}, len(columns))
		scanTo := make([]interface{}, len(columns))
		for i := range buffer {
			scanTo[i] = &buffer[i]
		}
		for rows.Next() {
			if err := rows.Scan(scanTo...); err != nil {
				return errors.Wrap(err, "failed to scan row")
			}
			qdb.Body(scanTo)
		}
		if !rows.NextResultSet() {
			break
		}
	}

	return nil
}

func (dx *DBX) query(statement string, args ...interface{}) error {
	if DatabaseDisabled() {
		return ErrDatabaseUnavailable
	}
	if dx.w == nil {
		dx.w = os.Stdout
	}
	rows, err := dx.db.Query(statement)
	if err != nil {
		return errors.Wrap(err, "dbx query failed")
	}
	defer rows.Close()
	flags := tabwriter.TabIndent
	if dx.lines {
		flags |= tabwriter.Debug
	}

	for {
		// tabwriter args: output, minwidth, tabwidth, padding, padchar, flags
		tw := tabwriter.NewWriter(
			dx.w,  // io.Writer
			0,     // min width
			0,     // tab width
			1,     // padding
			' ',   // pad character
			flags, // behavior flags
		)
		columns, _ := rows.Columns()

		if dx.header {
			for i, column := range columns {
				if i > 0 {
					fmt.Fprint(tw, "\t")
				}
				fmt.Fprint(tw, column)
			}
			fmt.Fprintln(tw)
			for i, column := range columns {
				if i > 0 {
					fmt.Fprint(tw, "\t")
				}
				fmt.Fprint(tw, strings.Repeat("-", len(column)))
			}
			fmt.Fprintln(tw)
		}

		buffer := make([]interface{}, len(columns))
		scanTo := make([]interface{}, len(columns))
		for i := range buffer {
			scanTo[i] = &buffer[i]
		}
		for rows.Next() {
			if err := rows.Scan(scanTo...); err != nil {
				tw.Flush()
				return errors.Wrap(err, "failed to scan row")
			}
			for i, column := range buffer {
				if i > 0 {
					fmt.Fprint(tw, "\t")
				}
				fmt.Fprint(tw, column)
			}
			fmt.Fprintln(tw)
		}
		tw.Flush()

		if !rows.NextResultSet() {
			break
		}
	}

	return nil
}

// run a query with a single column result and return the value of same
func queryColumn(db *sql.DB, statement string, args ...interface{}) (string, error) {
	var value string
	row := db.QueryRow(statement, args...)
	err := row.Scan(&value)
	return value, err
}

// run a single command and print its results
func dbCmd(ctx context.Context, dbName string, cluster []string, logger client.LogFunc, header, divs bool, statement string) error {
	dx, err := NewConnection(ctx, dbName, cluster, logger)
	if err != nil {
		return err
	}
	defer dx.Close()
	dx.header = header
	dx.lines = divs
	return dx.Eval(statement)
}

func getDB(ctx context.Context, dbName string, cluster []string, logger client.LogFunc) (*sql.DB, error) {
	dbxMu.Lock()
	defer dbxMu.Unlock()
	if db, ok := dbxDB[dbName]; ok {
		return db, nil
	}
	store := getStore(ctx, cluster)
	if len(dbxDB) == 0 {
		if logger == nil {
			logger = client.DefaultLogFunc
		}
		logOpt := driver.WithLogFunc(logger)
		dbDriver, err := driver.New(store, logOpt)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create dqlite driver")
		}
		sql.Register("dqlite", dbDriver)
	}

	db, err := sql.Open("dqlite", dbName)
	if err == nil {
		dbxDB[dbName] = db
	}
	db.SetMaxOpenConns(1) // dqlite is single-threaded
	return db, err
}

// Eval executes a single read/write query
func (dx *DBX) Eval(statement string) error {
	if statement == "" {
		return fmt.Errorf("no statements given")
	}
	// TODO: add additional sqlite cli commands
	switch statement {
	case ".schema":
		statement = "select sql || ';' from sqlite_master"
	case ".tables":
		statement = "select name from sqlite_master where type='table' order by name"
	}

	action := strings.ToUpper(strings.Fields(statement)[0])
	switch action {
	case "SELECT":
		return errors.Wrapf(dx.query(statement), "dx.Eval query failed: %q", statement)
	case "PRAGMA":
		// some pragmas write, others read...
		if strings.Contains(statement, "=") {
			_, err := dx.exec(statement)
			return errors.Wrapf(err, "dx.Eval pragma exec failed: %q", statement)
		}
		return errors.Wrapf(dx.query(statement), "dx.Eval pragma query failed: %q", statement)
	default:
		// Everything else is writing, e.g., INSERT, UPDATE, DELETE
		_, err := dx.exec(statement)
		return errors.Wrapf(err, "dx.Eval exec fail (%T): %q", err, statement)
	}
}

// execute a write statement against the database
func (dx *DBX) exec(query string, args ...interface{}) (result sql.Result, err error) {
	if DatabaseDisabled() {
		err = ErrDatabaseUnavailable
		return
	}

	const retryLimit = 10 // TODO: make configurable
	delay := time.Millisecond
	for i := 0; i < retryLimit; i++ {
		if result, err = dx.db.Exec(query, args...); err == nil {
			return
		}
		log.Printf("DBX exec (%d/%d::%T) err: %v\n", i+1, retryLimit, err, err)
		if derr, ok := err.(driver.Error); ok {
			// don't retry if its an actual sql failure
			if derr.Code > 0 {
				return
			}
		}
		time.Sleep(delay)
		// back of requests with simple geomtric series
		delay += delay

	}
	err = errors.Wrapf(err, "DBX exec fail")
	return
}

// Queryor is the database query functionality required
type Queryor interface {
	QueryRows(query string, args ...interface{}) ([]Rows, error)
	Close() error
}

// SqliteError represents an error that came from sqlite itself
type SqliteError interface {
	SqliteError() (int, string)
}

// DBX is the database handler used by dqlited
type DBX struct {
	db      *sql.DB
	name    string
	w       io.Writer
	header  bool
	lines   bool
	verbose bool
}

// NewConnection return a db connection
func NewConnection(ctx context.Context, dbName string, cluster []string, logger LogFunc) (*DBX, error) {
	db, err := getDB(ctx, dbName, cluster, logger)
	if err != nil {
		return nil, err
	}
	return &DBX{db: db, name: dbName, w: os.Stdout, header: true}, nil
}

// Close will close the open database connection
func (dx *DBX) Close() error {
	dbxMu.Lock()
	defer dbxMu.Unlock()
	delete(dbxDB, dx.name)
	return dx.db.Close()
}

// QueryRows returns the rows of a query
func (dx *DBX) QueryRows(query string, args ...interface{}) ([]Rows, error) {
	if DatabaseDisabled() {
		return nil, ErrDatabaseUnavailable
	}
	log.Printf("QUERY: %s ARGS: %v\n", query, args)
	reply := make([]Rows, 0, 32)
	action := strings.ToUpper(strings.Fields(query)[0])
	if action != "SELECT" && action != "PRAGMA" {
		return nil, fmt.Errorf("Invalid action: %q -- must use SELECT", action)
	}
	rows, err := dx.db.Query(query)
	if err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	defer rows.Close()
	var resp Rows
	resp.Columns, _ = rows.Columns()
	resp.Types = make([]string, len(resp.Columns))
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, errors.Wrap(err, "column types fail")
	}
	for i, colType := range colTypes {
		//fmt.Printf("COL TYPES (%d): %q SCAN: %v\n", i, colType.DatabaseTypeName(), colType.ScanType())
		resp.Types[i] = colType.DatabaseTypeName()
	}
	// TODO: add support for NextResultSet()
	for rows.Next() {
		// TODO: optimize by scanning directly into new resp.Rows
		//buffer := make([]string, len(resp.Columns))
		buffer := make([]interface{}, len(resp.Columns))
		scanTo := make([]interface{}, len(buffer))
		for i := range buffer {
			scanTo[i] = &buffer[i]
		}
		if err := rows.Scan(scanTo...); err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}
		fmt.Printf("SCANNED: %v\n", buffer)
		resp.Values = append(resp.Values, buffer)
	}
	reply = append(reply, resp)
	return reply, nil
}

// Result is the results of a database execution
// used by the web api for the python DBI adapter
type Result struct {
	LastInsertID int64   `json:"last_insert_id,omitempty"`
	RowsAffected int64   `json:"rows_affected,omitempty"`
	Error        string  `json:"error,omitempty"`
	Time         float64 `json:"time,omitempty"`
}

// Rows represents the outcome of an operation that returns query data.
// used by the web api for the python DBI adapter
type Rows struct {
	Columns []string        `json:"columns,omitempty"`
	Types   []string        `json:"types,omitempty"`
	Values  [][]interface{} `json:"values,omitempty"`
	Error   string          `json:"error,omitempty"`
	Time    float64         `json:"time,omitempty"`
}

// ExecuteResponse is the response used by pydqlite
type ExecuteResponse struct {
	Results []Result `json:"results,omitempty"`
	Time    float64  `json:"time,omitempty"`
}

// Executor interface abstracts database execution
type Executor interface {
	Execute(statements ...string) (*ExecuteResponse, error)
}

type DBServer interface {
	Executor
	Queryor
}

// Execute will execute a series of statements, exec and query
// TODO: consolidate with Batch?
func (dx *DBX) Execute(statements ...string) (*ExecuteResponse, error) {
	if DatabaseDisabled() {
		return nil, ErrDatabaseUnavailable
	}
	started := time.Now()
	results := make([]Result, 0, len(statements))

	for i, statement := range statements {
		resp, err := dx.db.Exec(statement)
		if err != nil {
			log.Printf("EXEC FAIL FOR: %q -- %v\n", statement, err)
			return nil, errors.Wrapf(err, "DBX.Execute fail (%d/%d): %q", i+1, len(statements), statement)
		}
		lastID, _ := resp.LastInsertId()
		affected, _ := resp.RowsAffected()
		if dx.verbose {
			log.Printf("EXEC OK (%d): %s\n", affected, statement)
		}
		result := Result{LastInsertID: lastID, RowsAffected: affected}
		results = append(results, result)
	}

	delta := time.Now().Sub(started).Seconds()
	return &ExecuteResponse{Results: results, Time: delta}, nil
}

type FileSaver func(files ...client.File) error

func FileWriter(files ...client.File) error {
	for _, file := range files {
		f, err := os.Create(file.Name)
		if err != nil {
			return errors.Wrapf(err, "create failed for file: %s", file.Name)
		}
		defer f.Close()
		if _, err = f.Write(file.Data); err != nil {
			return errors.Wrap(err, "write failed")
		}
	}
	return nil
}

// dbDumper is a generic database dump handler
func dbDumper(ctx context.Context, fs FileSaver, dbname string, cluster []string) error {
	client, err := getLeader(ctx, cluster)
	if err != nil {
		return errors.Wrap(err, "can't get leader")
	}
	files, err := client.Dump(ctx, dbname)
	if err != nil {
		return errors.Wrap(err, "client dump failed")
	}
	if err := fs(files...); err != nil {
		return errors.Wrap(err, "database dump failed")
	}
	log.Println("dump complete:", dbname)
	return nil
}

func dbDump(ctx context.Context, dbname string, cluster []string) error {
	return dbDumper(ctx, FileWriter, dbname, cluster)
}

func _dbDump(ctx context.Context, dbname string, cluster []string) error {
	client, err := getLeader(ctx, cluster)
	if err != nil {
		return errors.Wrap(err, "can't get leader")
	}
	log.Println("dumping:", dbname)
	files, err := client.Dump(ctx, dbname)
	if err != nil {
		return errors.Wrap(err, "dump failed")
	}
	for _, file := range files {
		f, err := os.Create(file.Name)
		if err != nil {
			return errors.Wrapf(err, "create failed for file: %s", file.Name)
		}
		if _, err = f.Write(file.Data); err != nil {
			return errors.Wrap(err, "write failed")
		}
		f.Close()
	}
	log.Println("dump complete:", dbname)
	return nil
}

// for one-shot file loads
func dbFile(ctx context.Context, filename, dbname string, batched, verbose bool, cluster []string) error {
	dx, err := NewConnection(ctx, dbname, cluster, nil)
	if err != nil {
		return err
	}
	defer dx.Close()
	dx.verbose = verbose
	defer dx.db.Close()
	return dx.loadFile(filename, batched)
}

// for one-shot process file with multiple queries
func dbReport(ctx context.Context, filename, dbname string, header, lines bool, cluster []string) error {
	dx, err := NewConnection(ctx, dbname, cluster, nil)
	if err != nil {
		return err
	}
	dx.header = header
	dx.lines = lines
	defer dx.db.Close()
	return dx.queryFile(filename)
}

// a line generator for strings
func lister(args ...string) chan string {
	c := make(chan string)
	go func() {
		for _, arg := range args {
			c <- arg
		}
		close(c)
	}()
	return c
}

// a line generator for a Reader
func liner(r io.ReadCloser) chan string {
	c := make(chan string)
	scanner := bufio.NewScanner(r)

	go func() {
		for scanner.Scan() {
			c <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			log.Println("reading standard input:", err)
		}
		close(c)
		r.Close()
	}()
	return c
}

// loadFile will apply the given file to the current database
func (dx *DBX) loadFile(fileName string, batched bool) error {
	if dx.verbose {
		log.Println("loading file:", fileName)
	}
	if batched {
		f, err := os.Open(fileName)
		if err != nil {
			return err
		}
		defer f.Close()
		return transact(dx.db, dx.verbose, liner(f))
	}
	buffer, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrapf(err, "error reading file: %s", fileName)
	}
	return dx.Batch(string(buffer))
}

func (dx *DBX) queryFile(fileName string) error {
	buffer, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrapf(err, "error reading file: %s", fileName)
	}
	return dx.query(string(buffer))
}

// transact executes the collection of statements as a single transaction
// This is critical for loading backups that may have tens of thousands
// of insert statements (or more) -- a normal "Exec" treats each insert
// as a separate transaction and can operate an order of magnitude slower
func transact(db *sql.DB, verbose bool, statements chan string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	count := 0
	started := time.Now()
	for s := range statements {
		if _, err := tx.Exec(s); err != nil {
			tx.Rollback()
			return err
		}
		count++
		if verbose {
			log.Println("success:", count)
		}
	}
	tx.Commit()
	//if verbose && count > 0 {
	if count > 0 {
		delta := time.Now().Sub(started)
		//ns := time.Now().Sub(started).Nanoseconds()
		rate := time.Duration(delta.Nanoseconds() / int64(count))
		log.Printf("success for: %d / %s = %s\n", count, delta, rate)
	}
	return nil
}

var (
	commentC   = regexp.MustCompile(`(?s)/\*.*?\*/`)
	commentSQL = regexp.MustCompile(`\s*--.*`)
	readline   = regexp.MustCompile(`(\.[a-z]+( .*)*)`)
)

// startsWith provides a case-insensitive string prefix test
func startsWith(data, sub string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(data)), strings.ToUpper(sub))
}

// CleanText removes C-style and SQL-style comments from the given text
// which allows for simpler parsing of the contents
func CleanText(s string) string {
	clean := commentC.ReplaceAll([]byte(s), []byte{})
	clean = commentSQL.ReplaceAll(clean, []byte{})
	return string(clean)
}

// Batch emulates the client reading a series of commands,
// primarily those created from dumping from sqlite.
//
// Input is as a string (instead of a Reaader) to allow for easy
// regexp across multiple lines. For
//
// Normally statements are terminated by ";" but this is complicated
// by trigger statements which include one or more statements with
// their own ";" occurances between the BEGIN and END
func (dx *DBX) Batch(buffer string) error {
	w := dx.w
	if w == nil {
		w = os.Stdout
	}
	// evaluate input on a line by line basis,
	// (though statements can be multiple lines)
	lines := strings.Split(CleanText(buffer), "\n")
	var trigger bool
	var err error
	var statement strings.Builder
	var tx *sql.Tx
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		//log.Println("LINE:", line)
		statement.WriteString(line + "\n")
		switch {
		case startsWith(line, "BEGIN TRANSACTION"):
			// skip the BEGIN line since it's implicit in the actual TX
			statement.Reset()
			if dx.verbose {
				log.Println(line)
			}
			tx, err = dx.db.Begin()
			if err != nil {
				return errors.Wrap(err, "could not create transaction")
			}
			continue
		case startsWith(line, "COMMIT;"):
			if dx.verbose {
				log.Println(line)
			}
			if err := tx.Commit(); err != nil {
				return errors.Wrap(err, "could not close transaction")
			}
			tx = nil
			statement.Reset()
			continue
		case startsWith(line, "CREATE TRIGGER"):
			if !strings.Contains(line, ";") {
				trigger = true
				continue
			}
		case startsWith(line, "END;"):
			trigger = false
		case trigger:
			log.Println("IN TRIGGER")
			continue
		}
		if !strings.Contains(line, ";") {
			continue
		}
		stmt := statement.String()
		switch {
		case startsWith(stmt, "SELECT"):
			dx.query(stmt)
		case tx != nil:
			if dx.verbose {
				log.Println("TX EXEC:", stmt)
			}
			if _, err := tx.Exec(stmt); err != nil {
				return err
			}
		default:
			if dx.verbose {
				log.Println("DB EXEC:", stmt)
			}
			if _, err := dx.exec(stmt); err != nil {
				log.Println("EXEC ERR:", err)
				return errors.Wrapf(err, "EXEC QUERY: %s FILE: %s", line, "mydb")
			}
		}
		statement.Reset()
	}
	return nil
}
