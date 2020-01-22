package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/canonical/go-dqlite/client"
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
	log.Println("making NewLogger with level:", level)
	return func(l client.LogLevel, format string, a ...interface{}) {
		// log levels start at 0 for Debug and increase up to Error
		// only print logs within that limit
		if l >= level {
			log.Printf("HIT:: "+format, a...)
		} else {
			log.Printf("MISSED: "+format, a...)
		}
	}
}

// DefaultLogger returns a logger using the default settings
func DefaultLogger(w io.Writer) client.LogFunc {
	return NewLogger(defaultLogLevel, w)
}

func getLeader(ctx context.Context, cluster []string) (*client.Client, error) {
	store := getStore(ctx, cluster)
	return client.FindLeader(ctx, store, client.WithLogFunc(NewLogger(defaultLogLevel, log.Writer())))
}

func getStore(ctx context.Context, cluster []string) client.NodeStore {
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
	store.Set(ctx, infos)
	return store
}
