package main

import (
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var (
	commentC   = regexp.MustCompile(`(?s)/\*.*?\*/`)
	commentSQL = regexp.MustCompile(`\s*--.*`)
)

func startsWith(data, sub string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(data)), strings.ToUpper(sub))
}

func listTables(db *sql.DB, w io.Writer) error {
	q := `
SELECT name FROM sqlite_master
WHERE type='table'
ORDER BY name
`
	return query(db, w, q)
}

func batchCommand(echo bool, fileName, dbName string, cluster []string) error {
	db, err := getDB(dbName, cluster)
	if err != nil {
		return errors.Wrap(err, "getDB failed")
	}
	defer db.Close()

	buffer, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", fileName)
	}
	return Batch(db, string(buffer), echo, os.Stdout)
}

// Batch applies a batch of sql statements
// TODO: use io.Reader instead of buffer string?
func Batch(db *sql.DB, buffer string, echo bool, w io.Writer) error {
	if w == nil {
		w = os.Stdout
	}
	// strip comments
	clean := commentC.ReplaceAll([]byte(buffer), []byte{})
	clean = commentSQL.ReplaceAll(clean, []byte{})

	lines := strings.Split(string(clean), "\n")
	multiline := "" // triggers are multiple lines
	trigger := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if 0 == len(line) {
			continue
		}
		if echo {
			fmt.Println("CMD>", line)
		}
		switch {
		case strings.HasPrefix(line, ".echo "):
			echo, _ = strconv.ParseBool(line[6:])
			continue
		/*
			case strings.HasPrefix(line, ".read "):
				name := strings.TrimSpace(line[6:])
				if err := File(db, name, echo, w); err != nil {
					return errors.Wrapf(err, "read file: %s", name)
				}
				continue
		*/
		case strings.HasPrefix(line, ".print "):
			str := strings.TrimSpace(line[7:])
			str = strings.Trim(str, `"`)
			str = strings.Trim(str, "'")
			fmt.Fprintln(w, str)
			continue
		case strings.HasPrefix(line, ".tables"):
			if err := listTables(db, w); err != nil {
				return errors.Wrapf(err, "table error")
			}
			continue
		case startsWith(line, "CREATE TRIGGER"):
			multiline = line
			trigger = true
			continue
		case startsWith(line, "END;"):
			line = multiline + "\n" + line
			multiline = ""
			trigger = false
		case trigger:
			multiline += "\n" + line // restore our 'split' transaction
			continue
		}
		if len(multiline) > 0 {
			multiline += "\n" + line // restore our 'split' transaction
		} else {
			multiline = line
		}
		if strings.Index(line, ";") < 0 {
			continue
		}
		if startsWith(multiline, "SELECT") {
			if err := query(db, w, line); err != nil {
				return errors.Wrapf(err, "query failed for: %q", line)
			}
		} else if _, err := db.Exec(multiline); err != nil {
			return errors.Wrapf(err, "EXEC STATEMENT: %s", line)
		}
		multiline = ""
	}
	return nil
}
