package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	dqlite "github.com/canonical/go-dqlite"
	dqclient "github.com/canonical/go-dqlite/client"
	"github.com/pkg/errors"
)

var (
	defaultCluster = []string{
		"127.0.0.1:9181",
		"127.0.0.1:9182",
		"127.0.0.1:9183",
	}
)

func clusterShow() error {
	client, err := getLeader(defaultCluster)
	if err != nil {
		return errors.Wrap(err, "can't connect to cluster leader")
	}
	defer client.Close()

	// TODO: make timeout configurable?
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var leader *dqclient.NodeInfo
	var nodes []dqclient.NodeInfo
	if leader, err = client.Leader(ctx); err != nil {
		return errors.Wrap(err, "can't get leader")
	}

	if nodes, err = client.Cluster(ctx); err != nil {
		return errors.Wrap(err, "can't get cluster")
	}

	fmt.Printf("ID \tLeader \tAddress\n")
	for _, node := range nodes {
		fmt.Printf("%d \t%v \t%s\n", node.ID, node.ID == leader.ID, node.Address)
	}
	return nil
}

func clusterAdd(id int, address string, cluster []string) error {
	if id == 0 {
		return fmt.Errorf("ID must be greater than zero")
	}
	if address == "" {
		address = fmt.Sprintf("127.0.0.1:918%d", id)
	}
	info := dqclient.NodeInfo{
		ID:      uint64(id),
		Address: address,
	}

	client, err := getLeader(cluster)
	if err != nil {
		return errors.Wrap(err, "can't connect to cluster leader")
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := client.Add(ctx, info); err != nil {
		return errors.Wrap(err, "can't add node")
	}

	return nil
}

const defaultNetworkLatency = 20 * time.Millisecond

func dbStart(id int, dir, address string) error {
	if id == 0 {
		return fmt.Errorf("ID must be greater than zero")
	}
	if address == "" {
		address = fmt.Sprintf("127.0.0.1:918%d", id)
	}
	dir = filepath.Join(dir, fmt.Sprint(id))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrapf(err, "can't create %s", dir)
	}
	node, err := dqlite.New(
		uint64(id), address, dir,
		dqlite.WithBindAddress(address),
		dqlite.WithNetworkLatency(defaultNetworkLatency),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create node")
	}
	if err := node.Start(); err != nil {
		return errors.Wrap(err, "failed to start node")
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGTERM)
	<-ch

	if err := node.Close(); err != nil {
		return errors.Wrap(err, "failed to stop node")
	}

	return nil
}
