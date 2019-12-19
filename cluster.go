package main

import (
	"context"
	"fmt"
	"log"
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

func clusterShow(address string, cluster ...string) error {
	client, err := getLeader(cluster)
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
	if id < 1 {
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

func dumper(client *dqclient.Client) {
	ch2 := make(chan os.Signal)
	signal.Notify(ch2, syscall.SIGUSR1)
	go func(cc chan os.Signal) {
		ctx := context.Background()
		for {
			<-cc
			leader, err := client.Leader(ctx)
			if err != nil {
				log.Println("leader error:", err)
			} else {
				log.Println("LEADER:", leader)
			}

			nodes, err := client.Cluster(ctx)
			if err != nil {
				log.Println("cluster error:", err)
				continue
			}
			for _, node := range nodes {
				log.Println(node)
			}
		}
	}(ch2)
}

func dbStart(id int, dir, address, filename string, cluster ...string) error {
	if id == 0 {
		return fmt.Errorf("ID must be greater than zero")
	}
	if address == "" {
		address = fmt.Sprintf("127.0.0.1:918%d", id)
	}
	log.Printf("starting node: %d -- listening on %q\n", id, address)
	dir = filepath.Join(dir, fmt.Sprint(id))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrapf(err, "can't create dir %s", dir)
	}
	node, err := dqlite.New(
		uint64(id), address, dir,
		dqlite.WithBindAddress(address),
		dqlite.WithNetworkLatency(defaultNetworkLatency),
		//dqlite.WithLogFunc(logFunc),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create node")
	}
	if err := node.Start(); err != nil {
		return errors.Wrap(err, "failed to start node")
	}

	client, err := getLeader(cluster)
	if err != nil {
		return errors.Wrap(err, "can't connect to cluster leader")
	}
	dumper(client)
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGTERM)
	sig := <-ch

	log.Printf("server id: %d is shutting down on signal: %v\n", id, sig)

	if err := client.Remove(context.Background(), uint64(id)); err != nil {
		log.Printf("error removing cluster id:%d error:%v\n", id, err)
	}
	if err := node.Close(); err != nil {
		return errors.Wrap(err, "failed to stop node")
	}
	log.Printf("server id: %d has shut down\n", id)

	return nil
}

func seeker(cluster ...string) error {
	client, err := getLeader(cluster)
	if err != nil {
		return errors.Wrap(err, "can't connect to cluster leader")
	}
	defer client.Close()

	var leader *dqclient.NodeInfo
	var nodes []dqclient.NodeInfo

	delay := time.Second

	for {
		ctx, cancel := context.WithTimeout(context.Background(), delay)

		log.Println("get leader")
		if leader, err = client.Leader(ctx); err != nil {
			log.Println("can't get leader", err)
			if client, err = getLeader(cluster); err != nil {
				log.Println("total failure:", err)
				//return err
			}
			goto sleep
		}

		if nodes, err = client.Cluster(ctx); err != nil {
			log.Println("can't get cluster", err)
			goto sleep
		}

		log.Printf("ID \tLeader \tAddress\n")
		for _, node := range nodes {
			log.Printf("%d \t%v \t%s\n", node.ID, node.ID == leader.ID, node.Address)
		}

	sleep:
		cancel()
		fmt.Printf("sleep (%s)\n", delay)
		time.Sleep(delay)
	}

	return nil
}
