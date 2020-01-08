package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/paulstuart/envy"
	"github.com/spf13/cobra"
)

const (
	insert = "insert into conflicted (server, msg, given) values(?, ?, ?)"
	upsert = "insert into overlay (server, msg, given) values(?, ?, ?)"
	simple = "insert into simple (server) values(?)"
	remove = "delete from simple where id=?"
)

// run a load test against the database
func newHammer() *cobra.Command {
	var cluster []string
	var dbName string
	var id int
	var count int

	cmd := &cobra.Command{
		Use:   "hammer",
		Short: "load test the database.",
		RunE: func(cmd *cobra.Command, args []string) error {
			hammer(id, count, dbName, cluster...)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.IntVarP(&count, "count", "n", 0, "how many times to repeat (0 is infinite)")
	flags.IntVarP(&id, "id", "i", envy.IntDefault("DQLITED_ID", 1), "server id")

	return cmd
}

func (dx *dbx) stretch(count int) {
	for i := 0; i < count; i++ {
		res, err := dx.exec(simple, i)
		if err != nil {
			panic(err)
		}
		rowid, err := res.LastInsertId()
		if err != nil {
			panic(err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			panic(err)
		}

		_, err = dx.exec(remove, rowid)
		if err != nil {
			panic(err)
		}
		log.Printf("OK %s - %d / %d\n", simple, rowid, affected)
	}
}

func hammer(id, count int, dbName string, cluster ...string) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	dx, err := NewConnection(dbName, cluster)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("connected (%d)\n", id)
	defer dx.db.Close()

	log.Println("strech count:", count)
	dx.stretch(count)
	log.Println("streched")
	/*
		fails := 0
			retry:
				if _, err := db.Exec(insert, id, "-none-", time.Now()); err != nil {
					log.Printf("insert error (%d/%d): %+v\n", id, fails, err)
					if fails++; fails < 5 {
						time.Sleep(time.Second)
						goto retry
					}
					log.Fatalf("I give up (%d)\n", id)
				}
	*/
	started := time.Now()
	fmt.Println("STARTING COUNT:", count)
	starting := count
	last := 0
	var total int64

	if starting > 0 {
		defer func() {
			//delta := time.Duration((time.time.Now().Sub(started).Nanoseconds() / count) * time.Nanosecond)
			delta := time.Now().Sub(started).Nanoseconds()
			//log.Printf("Averaged rate per exec: %s\n", time.Now().Sub(started)/count)
			rate := time.Duration(delta / int64(starting))
			//log.Printf("Averaged rate per exec: %s\n", time.Duration(delta/count))
			log.Printf("completed %d/%d -- average rate per exec: %s\n", total, last, rate)
			//log.Printf("Averaged rate per exec: %s\n", rate)
		}()
	}

	for {
		msg := "nada"
		if false {
			delay := rand.Intn(100)
			msg = fmt.Sprintf("delay: %dms", delay)
			time.Sleep(time.Millisecond * time.Duration(delay))
			print(msg)
		}
		//if _, err := db.Exec(upsert, id, msg, time.Now()); err != nil {
		//if _, err := db.Exec(simple, id); err != nil {
		//log.Println("exec:", simple, id)
		if r, err := dx.exec(simple, id); err != nil {
			log.Printf("hammer (%d) exec error : %+v\n", id, err)
		} else {
			if affected, err := r.RowsAffected(); err == nil {
				total += affected
			}
			/*
				if err != nil {
					log.Println("hammer rows error:", err)
				} else {
					log.Println("hammer rows affected:", affected)
				}
			*/
		}
		last++
		if count > 0 {
			count--
			//log.Printf("completed %d/%d\n", starting-count, starting)
			if count == 0 {
				break
			}
		}
	}
	log.Printf("completed %d/%d\n", total, last)
}
