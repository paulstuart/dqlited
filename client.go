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

var defaultLogLevel = client.LogError

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

// NewLogger returns a logging function used by dqlite
func NewLogger(level client.LogLevel, w io.Writer) client.LogFunc {
	if w == nil {
		w = os.Stdout
	}
	log.Printf("creating new logger that runs at level: %d\n", level)
	return func(l client.LogLevel, format string, a ...interface{}) {
		// log levels start at 0 for Debug and increase up to Error
		// only print logs within that limit
		if l >= level {
			log.Printf(format, a...)
		}
	}
}

// DefaultLogger returns a logger using the default settings
func DefaultLogger(w io.Writer) client.LogFunc {
	return NewLogger(defaultLogLevel, w)
}

func getLeader(timeout time.Duration, cluster []string) (*client.Client, error) {
	store := getStore(cluster)
	//log.Println("GET LEADER TIMEOUT:", timeout)
	//ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout) //2*time.Minute)
	defer cancel()

	return client.FindLeader(ctx, store, client.WithLogFunc(NewLogger(defaultLogLevel, os.Stdout)))
}

func getStore(cluster []string) client.NodeStore {
	//log.Println("GET STORE FOR CLUSTER:", cluster)
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
