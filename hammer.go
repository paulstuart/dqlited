package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	//"github.com/pkg/errors"
)

func hammerPrep(db *sql.DB) error {
	const drop = "drop table if exists simple"
	const create = "create table simple (id integer primary key, other integer)"
	if _, err := db.Exec(drop); err != nil {
		return err
	}
	_, err := db.Exec(create)
	return err
}

/*
type stackTracer interface {
	StackTrace() errors.StackTrace
}
*/

func hammerInserts(count int) string {
	var buf strings.Builder
	buf.WriteString("drop table if exists simple;\n")
	buf.WriteString("create table simple (id integer primary key, other integer)")
	for i := 1; i <= count; i++ {
		buf.WriteString(fmt.Sprintf("insert into simple (other) values(%d)", i))
	}
	return buf.String()
}

func hammer(kp *KeyPair, id, count int, logger LogFunc, dbName string, cluster ...string) {
	ctx := context.Background()
	fmt.Println("hammer using cluster:", cluster)
	dx, err := NewConnection(ctx, kp, dbName, cluster, logger)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("hammer connected")
	defer dx.db.Close()
	if err := hammerPrep(dx.db); err != nil {
		log.Fatalln(err)
	}
	log.Println("hammer prepped")
	hammerTime(dx.db, count)
	//hammerExec(dx.db, count)
}

func ts() string {
	const timeOut = "2006/01/02 03:04:05.000000PM -0700"
	return time.Now().Local().Format(timeOut)
}

func hammerTime(db *sql.DB, count int) {
	// get sub-second resolution
	const timeOut = "2006/01/02 03:04:05.000000PM -0700"
	var now string
	var fails, good, last int64

	started := time.Now().Local()
	fmt.Printf("%s hammerTime starting\n", started.Format(timeOut))
	for i := 0; i < count; i++ {
		// skip param binding so we can see the values in the sql statement
		//const insert = "insert into simple (other) values(?)"
		//resp, err := db.Exec(insert, last+1)
		insert := fmt.Sprintf("insert into simple (other) values(%d)", last+1)
		resp, err := db.Exec(insert)
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println("EOF error")
			} else {
				//for  _ ; err = errors.Unwrap(err); err != nil {
				for err != nil {
					fmt.Printf("sql error (%T): %+v\n", err, err)
					err = errors.Unwrap(err)
				}
			}
			fails++
			// first err (in a series?)
			if fails == 1 {
				now = time.Now().Format(timeOut)

			}
			fmt.Printf("%s fails: %3d (%d/%d):%T %-30s\r", now, fails, i, count, err, err.Error())
			continue
		}
		if id, err := resp.LastInsertId(); err != nil {
			fmt.Printf("%s failed to get insert id: %+v\n", now, err)
			continue
		} else {
			last = id
		}
		if fails > 0 {
			fmt.Printf("\n%s fixed: (%d/%d)\n", ts(), good, count)
			fails = 0
		}
		good++
		if last != good {
			fmt.Printf("%s offby: (%d/%d)\n", ts(), last, good)
			good = last // reset to avoid repeating same error
		}
		// visual reminder we're good
		if (good % 1000) == 0 {
			now = time.Now().Format(timeOut)
			fmt.Printf("%s  good: (%d/%d)\n", ts(), good, count)
		}
	}
	delta := time.Now().Sub(started)
	total := good + fails
	if total == 0 {
		fmt.Println("nada")
		return
	}
	per := delta / time.Duration(total)
	sec := (total * int64(time.Second)) / int64(delta)
	fmt.Printf("\ncompleted %d/%d in %s (%s/insert, %d/sec)\n", good, total, delta, per, sec)
}

// UNUSED FOR NOW

func hammerExec(db *sql.DB, count int) {
	fmt.Println("not in use")
	return
	done := false
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		bye := <-sig
		log.Printf("hammer is shutting down on signal: %v\n", bye)
		done = true
	}()
	const timeOut = "2006/01/02 03:04:05.000000PM -0700"
	var now string
	var fails, good, last int

	ctx := context.Background()

	started := time.Now().Local()
	fmt.Printf("%s test is starting\n", started.Format(timeOut))
	for i := 0; i < count; i++ {
		if done {
			break
		}
		// normally we use parameterized queries, but this makes it easier
		// to track issues via the sql statement for debugging
		insert := fmt.Sprintf("insert into simple (other) values(%d)", last+1)
		resp, err := db.ExecContext(ctx, insert)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SQL ERR (%T): %w\n", err, err)
			// first err (in a series?)
			fails++
			var nerr net.Error
			if errors.Is(err, io.EOF) {
				fmt.Println("End of the line, wait it out")
				time.Sleep(time.Second)
				continue
			} else if errors.As(err, &nerr) {
				fmt.Printf("network error!!! timeout:%t temp:%t msg:%s\n", nerr.Timeout(), nerr.Temporary(), err)
				continue
			}
			if fails == 1 {
				now = time.Now().Format(timeOut)
			} else {
				// reprint on same line to make real-time viewing easier
				if (fails % 10) == 0 {
					fmt.Print(" pause for a second...\n")
					time.Sleep(time.Second)
				}
				fmt.Printf("\r")
			}
			/*
				if false {
					if err, ok := err.(stackTracer); ok {
						st := err.StackTrace()
						fmt.Printf("DANGIT: %+v", st) // top two frames
					} else {
						fmt.Printf("err (%T) is not a stack tracer\n", err)
					}
				}
			*/

			/*
				cause := errors.Cause(err)
				if sqlErr, ok := cause.(SqliteError); ok {
					code, msg := sqlErr.SqliteError()
					fmt.Printf("Erp! SqliteError (%d): %s\n", code, msg)
				} else {
					fmt.Printf("%s fails: %3d (%d/%d):%T %-30s", now, fails, i, count, cause, err.Error())
				}
			*/
			continue
		}
		if id, err := resp.LastInsertId(); err != nil {
			fmt.Printf("%s failed to get insert id: %+v\n", now, err)
			continue
		} else {
			last = int(id)
		}
		good++
		if last != good {
			fmt.Printf("%s offby: (%d/%d)\n", now, last, good)
			good = last // reset to avoid repeating same error
		}
		if fails > 0 {
			fmt.Printf("\n%s fixed: (%d/%d)\n", now, good, count)
			fails = 0
			continue
		}
		// visual reminder we're good
		if (good % 1000) == 0 {
			now = time.Now().Format(timeOut)
			fmt.Printf("%s  good: (%d/%d)\n", now, good, count)
		}
	}
	delta := time.Now().Sub(started)
	total := good + fails
	per := delta / time.Duration(total)
	fmt.Printf("completed %d/%d (%s/insert)\n", good, total, per)
}
