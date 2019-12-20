package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/canonical/go-dqlite/driver"
	"github.com/pkg/errors"
)

func query(db *sql.DB, w io.Writer, lines bool, statement string, args ...interface{}) error {
	rows, err := db.Query(statement)
	if err != nil {
		return errors.Wrap(err, "query failed")
	}
	defer rows.Close()
	flags := tabwriter.TabIndent
	if lines {
		flags |= tabwriter.Debug
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', flags)

	columns, _ := rows.Columns()
	buffer := make([]interface{}, len(columns))
	scanTo := make([]interface{}, len(columns))
	for i := range buffer {
		scanTo[i] = &buffer[i]
	}

	// print header
	for _, column := range columns {
		fmt.Fprint(tw, column)
		fmt.Fprint(tw, "\t")
	}
	fmt.Fprintln(tw)
	for _, column := range columns {
		fmt.Fprint(tw, strings.Repeat("-", len(column)))
		fmt.Fprint(tw, "\t")
	}
	fmt.Fprintln(tw)

	for rows.Next() {
		if err := rows.Scan(scanTo...); err != nil {
			tw.Flush()
			return errors.Wrap(err, "failed to scan row")
		}
		for _, column := range buffer {
			fmt.Fprint(tw, column)
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
	return nil
}

func dbCmd(dbName string, cluster []string, divs bool, statements ...string) error {
	db, err := getDB(dbName, cluster)
	if err != nil {
		return err
	}
	defer db.Close()
	return dbExec(db, divs, statements...)
}

func getDB(dbName string, cluster []string) (*sql.DB, error) {
	store := getStore(cluster)
	driver, err := driver.New(store, driver.WithLogFunc(logFunc))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create dqlite driver")
	}
	sql.Register("dqlite", driver)

	return sql.Open("dqlite", dbName)
}

// all purpose wrapper (for now) TODO: rethink this
func dbExec(db *sql.DB, divs bool, statements ...string) error {
	if len(statements) == 0 {
		return fmt.Errorf("no statements given")
	}
	for i, statement := range statements {
		action := strings.ToUpper(strings.Fields(statement)[0])
		if action == "SELECT" || action == "PRAGMA" {
			if err := query(db, os.Stdout, divs, statement); err != nil {
				return errors.Wrapf(err, "dbExec query fail %d/%d", i+1, len(statements))
			}
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			return errors.Wrapf(err, "dbExec fail (%T) %d/%d", err, i+1, len(statements))
		}

	}
	return nil
}

type Queryor interface {
	QueryDB(queries string, args ...interface{}) ([]Rows, error)
	Close() error
}

type dbx struct {
	db *sql.DB
}

// NewConnection creates a db connection
func NewConnection(dbname string, cluster []string) (*dbx, error) {
	store := getStore(cluster)
	driver, err := driver.New(store, driver.WithLogFunc(logFunc))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create dqlite driver")
	}
	sql.Register("dqlite", driver)

	db, err := sql.Open("dqlite", dbname)
	if err != nil {
		return nil, errors.Wrap(err, "can't open database")
	}

	return &dbx{db}, nil
}

func (d *dbx) Close() error {
	return d.db.Close()
}

func (d *dbx) QueryDB(query string, args ...interface{}) ([]Rows, error) {
	log.Printf("QUERY: %s ARGS: %v\n", query, args)
	reply := make([]Rows, 0, 32)
	action := strings.ToUpper(strings.Fields(query)[0])
	if action != "SELECT" && action != "PRAGMA" {
		return nil, fmt.Errorf("Invalid action: %q -- must use SELECT", action)
	}
	rows, err := d.db.Query(query)
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
		fmt.Printf("COL TYPES (%d): %q SCAN: %v\n", i, colType.DatabaseTypeName(), colType.ScanType())
		resp.Types[i] = colType.DatabaseTypeName()
	}
	fmt.Printf("\nRESP TYPES: %v\n\n", resp.Types)
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

type Result struct {
	LastInsertID int64   `json:"last_insert_id,omitempty"`
	RowsAffected int64   `json:"rows_affected,omitempty"`
	Error        string  `json:"error,omitempty"`
	Time         float64 `json:"time,omitempty"`
}

// Rows represents the outcome of an operation that returns query data.
type Rows struct {
	Columns []string        `json:"columns,omitempty"`
	Types   []string        `json:"types,omitempty"`
	Values  [][]interface{} `json:"values,omitempty"`
	Error   string          `json:"error,omitempty"`
	Time    float64         `json:"time,omitempty"`
}

type ExecuteResponse struct {
	Results []Result `json:"results,omitempty"`
	Time    float64  `json:"time,omitempty"`
}

type Executor interface {
	Execute(statements ...string) (*ExecuteResponse, error)
}

func (d *dbx) Execute(statements ...string) (*ExecuteResponse, error) {
	// TODO: add back atomic/timing bits

	results := make([]Result, 0, len(statements))

	for i, statement := range statements {
		resp, err := d.db.Exec(statement)
		if err != nil {
			log.Printf("EXEC FAIL FOR: %q -- %v\n", statement, err)
			return nil, errors.Wrapf(err, "dbx.Execute fail (%d/%d): %q", i+1, len(statements), statement)
		}
		lastID, _ := resp.LastInsertId()
		affected, _ := resp.RowsAffected()
		log.Printf("EXEC OK (%d): %s\n", affected, statement)
		result := Result{LastInsertID: lastID, RowsAffected: affected}
		results = append(results, result)
	}

	return &ExecuteResponse{Results: results}, nil
}

func writeResponse(w http.ResponseWriter, r *http.Request, j *ExecuteResponse) {
	enc := json.NewEncoder(w)
	if pretty, _ := isPretty(r); pretty {
		enc.SetIndent("", "    ")
	}

	if err := enc.Encode(j); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func dbDump(filename, dbname string, cluster []string) error {
	client, err := getLeader(cluster)
	if err != nil {
		return errors.Wrap(err, "can't get leader")
	}
	log.Println("dumping:", dbname)
	files, err := client.Dump(context.Background(), dbname)
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
	return nil
}

// for one-shot file loads
func dbFile(filename, dbname string, cluster []string) error {
	db, err := getDB(dbname, cluster)
	if err != nil {
		return err
	}
	defer db.Close()
	return loadFile(db, filename)
}

func loadFile(db *sql.DB, fileName string) error {
	buffer, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrapf(err, "error reading file: %s", fileName)
	}
	_, err = db.Exec(string(buffer))
	return err
}
