package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	//"strings"
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

func nodeStart(id int, add bool, dir, address string, timeout time.Duration, cluster ...string) error {
	if id == 0 {
		return fmt.Errorf("ID must be greater than zero")
	}
	var bind string
	if address == "" {
		address = fmt.Sprintf("127.0.0.1:918%d", id)
		bind = address
		log.Printf("using default address: %q\n", address)
	} else {
		// we need to bind this address, so if it is name, we need to resolve it
		// TODO: easy sanity check to see if it's already a numeric address
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return errors.Wrapf(err, "failed to split address: %q", address)
		}
		ipaddr, err := net.ResolveIPAddr("ip", host)
		if err != nil {
			return errors.Wrapf(err, "failed to resolve host: %q", host)
		}
		bind = net.JoinHostPort(ipaddr.String(), port)
		log.Println("bind address is now:", bind)
	}
	log.Printf("creating node: %d @ %s -- listening on %q (ip:%s)\n", id, address, bind, myIP())
	log.Printf("cluster: %v\n", cluster)
	dir = filepath.Join(dir, fmt.Sprint(id))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrapf(err, "can't create dir %s", dir)
	}
	/*
		if !strings.HasPrefix(address, "@") {
			bits := strings.Split(address, ":")
			if len(bits) > 1 {
				address = "0.0.0.0:" + bits[1]
			} else {
				address = "0.0.0.0:" + address
			}
		}
		log.Printf("BIND ADDRESS: %s\n", address)
	*/
	node, err := dqlite.New(
		uint64(id), address, dir,
		dqlite.WithBindAddress(bind),
		dqlite.WithNetworkLatency(defaultNetworkLatency),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create node")
	}
	log.Printf("starting node: %d\n", id)
	if err := node.Start(); err != nil {
		return errors.Wrap(err, "failed to start node")
	}
	/*
		const wait = 1 * time.Second
		log.Printf("we're waiting %s for node to start\n", wait)
		time.Sleep(wait)
		log.Printf("ok get node leader via cluster: %s\n", cluster)
	*/
	client, err := getLeader(timeout, cluster)
	if err != nil {
		//log.Println("CRAPOLA:", err)
		return errors.Wrap(err, "can't connect to cluster leader")
	}

	if add {
		info := dqclient.NodeInfo{
			ID:      uint64(id),
			Address: address,
		}

		const addTimeout = time.Second * 10
		ctx, cancel := context.WithTimeout(context.Background(), addTimeout)
		defer cancel()

		log.Printf("add mode with timeout of: %s\n", addTimeout)
		if err := client.Add(ctx, info); err != nil {
			log.Printf("error adding node: %d (%s) to cluster: %v\n", id, address, err)
			return errors.Wrapf(err, "can't add node id: %d", id)
		}
	} else {
		log.Printf("skipping adding server: %d to cluster\n", id)
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT)
	signal.Notify(sig, syscall.SIGTERM)
	go func() {
		bye := <-sig
		log.Printf("server id: %d is shutting down on signal: %v\n", id, bye)

		if err := client.Remove(context.Background(), uint64(id)); err != nil {
			log.Printf("error removing cluster id:%d error:%v\n", id, err)
		}
		if err := node.Close(); err != nil {
			log.Println("failed to stop node:", err)
		}
		log.Printf("server id: %d has shut down\n", id)
		os.Exit(0)
	}()
	log.Printf("node %d has been started\n", id)
	return nil
}

func clusterShow(address string, timeout time.Duration, cluster ...string) error {
	client, err := getLeader(timeout, cluster)
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

// add node <id> to <cluster>
func clusterAdd(id int, address string, timeout time.Duration, cluster []string) error {
	if id < 1 {
		return fmt.Errorf("ID must be greater than zero")
	}
	if address == "" {
		return errors.New("address cannot be blank")
		//address = fmt.Sprintf("127.0.0.1:918%d", id)
	}
	info := dqclient.NodeInfo{
		ID:      uint64(id),
		Address: address,
	}

	client, err := getLeader(timeout, cluster)
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

// listen for SIGUSR1 and dump cluster info to log
func dumper(client *dqclient.Client) {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGUSR1)

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
	}(ch)
}

func dbStart(id int, dir, address, filename string, cluster ...string) error {
	if id == 0 {
		return fmt.Errorf("ID must be greater than zero")
	}
	if address == "" {
		address = fmt.Sprintf("127.0.0.1:918%d", id)
		log.Printf("using default address: %q\n", address)
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

	const timeout = time.Second * 60
	client, err := getLeader(timeout, cluster)
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

/*
func seeker(dbName, statement string, pause time.Duration, cluster ...string) error {
	db, err := getDB(dbName, cluster)
	if err != nil {
		return err
	}
	defer db.Close()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	for {
		value, err := queryColumn(db, statement)
		if err == nil {
			log.Println(value)
		} else {
			log.Println("query error:", err)
		}
		time.Sleep(pause)
	}

	return nil
}
*/
