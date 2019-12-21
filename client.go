package main

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/canonical/go-dqlite/client"
	//"github.com/canonical/go-dqlite/driver"
)

var logLevel = client.LogError

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

func NewLogger(level client.LogLevel, w io.Writer) client.LogFunc {
	if w == nil {
		w = os.Stdout
	}
	return func(l client.LogLevel, format string, a ...interface{}) {
		// log levels start at 0 for Debug and increase up to Error
		// only print logs within that limit
		if l >= level {
			log.Printf(format, a...)
		}
	}
}

func DefaultLogger(w io.Writer) client.LogFunc {
	return NewLogger(logLevel, w)
}
/*
func logFunc(l client.LogLevel, format string, a ...interface{}) {
	log.Printf(format, a...)
}
*/

func getLeader(cluster []string) (*client.Client, error) {
	store := getStore(cluster)

	//ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return client.FindLeader(ctx, store, client.WithLogFunc(NewLogger(logLevel, nil)))
}

func getStore(cluster []string) client.NodeStore {
	//log.Println("GET CLUSTER:", cluster)
	store := client.NewInmemNodeStore()
	if len(cluster) == 0 {
		cluster = defaultCluster
	}
	infos := make([]client.NodeInfo, len(cluster))
	for i, address := range cluster {
		infos[i].ID = uint64(i + 1)
		infos[i].Address = address
	}
	//log.Println("INFOS:", infos)
	store.Set(context.Background(), infos)
	return store
}
