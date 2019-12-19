package main

import (
	"context"
	"log"
	"time"

	"github.com/canonical/go-dqlite/client"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

func logFunc(l client.LogLevel, format string, a ...interface{}) {
	log.Printf(format, a...)
}

func getLeader(cluster []string) (*client.Client, error) {
	store := getStore(cluster)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return client.FindLeader(ctx, store, client.WithLogFunc(logFunc))
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
