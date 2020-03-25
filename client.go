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
	log.SetPrefix("BOOYAH")
}

// NewLogFunc returns a logging function used by dqlite
func NewLogFunc(level client.LogLevel, prefix string, w io.Writer) client.LogFunc {
	if w == nil {
		w = os.Stdout
	}
	log.Println("making NewLogger with level:", level)
	return func(l client.LogLevel, format string, a ...interface{}) {
		// log levels start at 0 for Debug and increase up to Error
		// only print logs within that limit
		if l >= level {
			log.Printf(prefix+format, a...)
		}
	}
}

// DefaultLogger returns a logger using the default settings
func DefaultLogger(w io.Writer) client.LogFunc {
	return NewLogFunc(defaultLogLevel, "", w)
	//return client.NewLogFunc(defaultLogLevel, "", w)
}

type logWriter struct{}

func (l *logWriter) Write(in []byte) (int, error) {
	log.Println(string(in))
	return len(in), nil
}

// NewLoggingWriter returns an io.Writer using the default Go logger
func NewLoggingWriter() io.Writer {
	return &logWriter{}
}

func getLeader(ctx context.Context, cluster []string) (*client.Client, error) {
	store := getStore(ctx, cluster)
	//return client.FindLeader(ctx, store, client.WithLogFunc(NewLogger(defaultLogLevel, log.Writer())))
	//logFunc := client.NewLogFunc(defaultLogLevel, "", log.Writer())
	logFunc := NewLogFunc(defaultLogLevel, "", nil)
	//return client.FindLeader(ctx, store, client.WithLogFunc(client.NewLogFunc(defaultLogLevel, "", log.Writer())))
	return client.FindLeader(ctx, store, client.WithLogFunc(logFunc))
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
