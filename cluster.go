package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	dqlite "github.com/canonical/go-dqlite"
	app "github.com/canonical/go-dqlite/app"
	dqclient "github.com/canonical/go-dqlite/client"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var (
	defaultCluster = []string{
		"127.0.0.1:9181",
		"127.0.0.1:9182",
		"127.0.0.1:9183",
	}
)

type KeyPair struct {
	Cert, Key string
}

// TODO: this is very wrong, but it's a placeholder at least
var (
	globalKeys KeyPair = KeyPair{"cluster.crt", "cluster.key"}
)

// Assign sets a new role for a node
func Assign(ctx context.Context, pair *KeyPair, id uint64, role dqclient.NodeRole, cluster []string) error {
	client, err := getLeader(ctx, pair, cluster)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Assign(ctx, id, role)
}

// LeaderID returns the id of the leader node, or 0 if unknown
func LeaderID(ctx context.Context, pair *KeyPair, cluster []string) (uint64, error) {
	client, err := getLeader(ctx, pair, cluster)
	if err != nil {
		return 0, errors.Wrap(err, "error getting client")
	}
	defer client.Close()
	leader, err := client.Leader(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "error getting leader")
	}
	return leader.ID, nil
}

// Transfer tries to transfer leadership to a new role
func Transfer(ctx context.Context, pair *KeyPair, id uint64, cluster []string) error {
	client, err := getLeader(ctx, pair, cluster)
	if err != nil {
		return err
	}
	defer client.Close()
	leader, err := client.Leader(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting leader")
	}
	log.Printf("transfer leadership from:%d to:%d\n", leader.ID, id)
	//return client.Transfer(ctx, id)
	err = client.Transfer(ctx, id)
	log.Printf("transferred leadership from:%d to:%d\n", leader.ID, id)
	return err
}

// Remove removes a node from the cluster
func Remove(ctx context.Context, pair *KeyPair, id uint64, cluster []string) error {
	client, err := getLeader(ctx, pair, cluster)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Remove(ctx, id)
}

// exists checks to see if the node already exists
func exists(ctx context.Context, client *dqclient.Client, id uint64, address string) bool {
	nodes, err := client.Cluster(ctx)
	if err != nil {
		panic(errors.Wrap(err, "can't get cluster"))
	}

	for _, node := range nodes {
		if node.ID == id {
			if address != node.Address {
				const msg = "mismatched addresses for node: %d (%q vs. %q)"
				panic(fmt.Sprintf(msg, id, address, node.Address))
			}
			return true
		}
	}
	return false
}

// statusFunc returns a function that yields cluster node status
func statusFunc(pair *KeyPair, cluster []string) func() ([]dqclient.NodeInfo, error) {
	ctx := context.Background()
	client, err := getLeader(ctx, pair, cluster)
	if err != nil {
		log.Fatalln("can't connect to cluster leader:", err)
	}
	return func() ([]dqclient.NodeInfo, error) {

		nodes, err := client.Cluster(ctx)
		if err == nil {
			sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
		}
		return nodes, nil
	}
}

// remove <s> from <list> if it is present
func omit(s string, list []string) []string {
	for i, item := range list {
		// match on port
		if strings.TrimLeft(s, ":") == strings.TrimLeft(item, ":") {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// start a dqlite node
func nodeStart(ctx context.Context, id uint64, dir, address string) (*dqlite.Node, error) {
	if id == 0 {
		return nil, fmt.Errorf("ID must be greater than zero")
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
			return nil, errors.Wrapf(err, "failed to split address: %q", address)
		}
		ipaddr, err := net.ResolveIPAddr("ip", host)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve host: %q", host)
		}
		bind = net.JoinHostPort(ipaddr.String(), port)
		log.Println("bind address is now:", bind)
	}
	dir = filepath.Join(dir, fmt.Sprint(id))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.Wrapf(err, "can't create dir %s", dir)
	}
	node, err := dqlite.New(
		id, address, dir,
		dqlite.WithBindAddress(bind),
		dqlite.WithNetworkLatency(defaultNetworkLatency),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create node")
	}

	log.Printf("starting node: %d\n", id)
	return node, errors.Wrap(node.Start(), "failed to start node")
}

// NodeRole is a go-dqlite Node role (e.g., voter, stand-by, or spare)
type NodeRole dqclient.NodeRole

const (
	// Voter node will replicate data and participate in quorum
	Voter = NodeRole(dqclient.Voter)

	// Standby node will replicate data but won't participate in quorum
	Standby = NodeRole(dqclient.StandBy)

	// Spare node won't replicate data and won't participate in quorum
	Spare = NodeRole(dqclient.Spare)
)

func nodeRole(s string) (NodeRole, error) {
	switch strings.ToLower(s) {
	case "voter":
		return Voter, nil
	case "standby", "stand-by":
		return Standby, nil
	case "spare":
		return Spare, nil
	}
	return NodeRole(255), fmt.Errorf("invalid role name: %q", s)
}

// ClientFunc is an interface to run commands from a CLI

type ClientFunc func(context.Context, *dqclient.Client) error

func withClient(ctx context.Context, pair *KeyPair, fn ClientFunc, cluster ...string) error {
	client, err := getLeader(ctx, pair, cluster)
	if err != nil {
		return errors.Wrap(err, "can't connect to cluster leader")
	}
	defer client.Close()
	return fn(ctx, client)
}

func clusterShow(ctx context.Context, pair *KeyPair, cluster ...string) error {
	return withClient(ctx, pair, show, cluster...)
}

func show(ctx context.Context, client *dqclient.Client) error {
	leader, err := client.Leader(ctx)
	if err != nil {
		return errors.Wrap(err, "can't get leader info")
	}
	nodes, err := client.Cluster(ctx)
	if err != nil {
		return errors.Wrap(err, "can't get cluster")
	}

	flags := tabwriter.TabIndent

	tw := tabwriter.NewWriter(
		os.Stdout, // io.Writer
		0,         // min width
		0,         // tab width
		1,         // padding
		' ',       // pad character
		flags,     // behavior flags
	)

	// TODO: make header optional
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	fmt.Fprintf(tw, "ID \tRole \tLeader \tAddress\t\n")
	fmt.Fprintf(tw, "-- \t---- \t------ \t-------\t\n")
	for _, node := range nodes {
		isLeader := node.ID == leader.ID
		fmt.Fprintf(tw, "%d \t%s \t%t \t%s\t\n", node.ID, node.Role, isLeader, node.Address)
	}
	tw.Flush()
	return nil
}

// add node <id> to <cluster>
func nodeAdd(ctx context.Context, client *dqclient.Client, id uint64, role NodeRole, address string) error {
	if id == 0 {
		return fmt.Errorf("ID must be greater than zero")
	}
	nodes, err := client.Cluster(ctx)
	if err != nil {
		return errors.Wrap(err, "get cluster nodes")
	}
	// don't add if we're already there
	for _, node := range nodes {
		if id == node.ID {
			if node.Address != address {
				const msg = "node:%d address: %q does not match existing address: %q"
				return fmt.Errorf(msg, id, address, node.Address)
			}
			log.Println("no need to add node:", id)
			return nil
		}
	}
	info := dqclient.NodeInfo{
		ID:      id,
		Address: address,
		Role:    dqclient.NodeRole(role),
	}
	return errors.Wrap(client.Add(ctx, info), "can't add node")
}

const defaultNetworkLatency = 20 * time.Millisecond

// StartServer provides a web interface to the database
// No error to return as it's never intended to stop
// TODO: too many args, consolidate into config struct
// TODO: is ctx n/a here?
func StartServer(ctx context.Context, id, port int, keyPair *KeyPair, dir, address string, cluster []string) error {
	log.Printf("starting server node:%d address:%q dir:%q ip:%s cluster:%v\n", id, address, dir, myIP(), cluster)

	// TODO: do we need to set up db now?
	dbname := ""
	if dbname != "" {
		dir = filepath.Join(dir, dbname)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrapf(err, "can't create %s", dir)
	}
	logfun := NewLogLog(dqclient.LogDebug)

	options := []app.Option{app.WithAddress(address), app.WithCluster(cluster), app.WithLogFunc(logfun)}

	/*
		crt := "cluster.crt"
		key := "cluster.key"
		if crt != "" {
	*/
	if keyPair != nil && keyPair.Cert != "" {
		cert, err := tls.LoadX509KeyPair(keyPair.Cert, keyPair.Key)
		if err != nil {
			return err
		}

		data, err := ioutil.ReadFile(keyPair.Cert)
		if err != nil {
			return err
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return fmt.Errorf("bad certificate")
		}

		listen, dial := app.SimpleTLSConfig(cert, pool)
		options = append(options, app.WithTLS(listen, dial))
	}
	dq, err := app.New(dir, options...)
	if err != nil {
		return errors.Wrap(err, "no new app for you!")
	}

	// TODO: add host option
	web := fmt.Sprintf("0.0.0.0:%d", port)
	m := http.NewServeMux()
	s := http.Server{Addr: web, Handler: m}
	for _, handler := range webHandlers(ctx, dq) {
		m.HandleFunc(handler.Path, handler.Func)
	}

	ch := make(chan os.Signal)

	m.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			fmt.Fprintln(w, "that method is the wrong one")
			return
		}
		fmt.Fprintln(w, "shutting down")
		ch <- unix.SIGQUIT
	})

	listener, err := net.Listen("tcp", web)
	if err != nil {
		return err
	}

	// TODO: add TLS
	go s.Serve(listener)

	signal.Notify(ch, unix.SIGPWR)
	signal.Notify(ch, unix.SIGINT)
	signal.Notify(ch, unix.SIGQUIT)
	signal.Notify(ch, unix.SIGTERM)

	sig := <-ch
	log.Println("shutting down on signal:", sig)

	listener.Close()
	s.Shutdown(context.Background())
	log.Println("clossing application")
	dq.Close()
	log.Println("application has shut down")

	return nil
}
