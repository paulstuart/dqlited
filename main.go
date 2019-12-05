package main

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const defaultDatabase = "demo.db"

func main() {
	cmd := path.Base(os.Args[0])
	root := newRoot(cmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// Return a new root command.
func newRoot(cmdName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdName,
		Short: "Manage dqlite servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		},
	}
	cmd.AddCommand(newStart())
	cmd.AddCommand(newAdd())
	cmd.AddCommand(newCluster())
	cmd.AddCommand(newAdhoc())
	cmd.AddCommand(newWeb())
	cmd.AddCommand(newDumper())
	cmd.AddCommand(newBatch())

	return cmd
}

// Start a web server for remote clients.
func newWeb() *cobra.Command {
	var cluster []string
	var dbName string
	var port int

	cmd := &cobra.Command{
		Use:   "webserver",
		Short: "Start a web server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			WebServer(port, defaultDatabase, cluster)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&dbName, "database", "d", defaultDatabase, "name of database to use")
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")
	flags.IntVarP(&port, "port", "p", 4001, "port to serve traffic on")

	return cmd
}

// Return a new start command.
func newStart() *cobra.Command {
	var dir string
	var address string

	cmd := &cobra.Command{
		Use:   "start <id>",
		Short: "Start a dqlite node.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrapf(err, "%s is not a number", args[0])
			}
			return dbStart(id, dir, address)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&dir, "dir", "d", "/tmp/dqlited", "base demo directory")
	flags.StringVarP(&address, "address", "a", "", "address of the node (default is 127.0.0.1:918<ID>)")

	return cmd
}

// Return a cluster nodes command.
func newCluster() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "display cluster nodes.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return clusterShow()
		},
	}
	return cmd
}

// Return a new add command.
func newAdd() *cobra.Command {
	var address string
	var cluster *[]string

	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add a node to the dqlite-demo cluster.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrapf(err, "%s is not a number", args[0])
			}
			return clusterAdd(id, address, *cluster)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&address, "address", "a", "", "address of the node to add (default is 127.0.0.1:918<ID>)")
	cluster = flags.StringSliceP("cluster", "c", defaultCluster, "addresses of existing cluster nodes")

	return cmd
}

// Return a new update key command.
func newAdhoc() *cobra.Command {
	var cluster []string
	var dbName string

	cmd := &cobra.Command{
		Use:   "adhoc <statment>...",
		Short: "execute a statement against the demo database.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dbExec(dbName, cluster, args...)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&dbName, "database", "d", defaultDatabase, "name of database to use")
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")

	return cmd
}

// Return a new dump command.
func newDumper() *cobra.Command {
	var cluster []string
	var dbName string

	cmd := &cobra.Command{
		Use:   "dump database",
		Short: "dump the Database and its associated WAL file.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dbDump(args[0], dbName, cluster)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&dbName, "database", "d", defaultDatabase, "name of database to use")
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")

	return cmd
}

// execute a batch of commands
func newBatch() *cobra.Command {
	var cluster []string
	var dbName string
	var fileName string
	var echo bool

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Execute the statements in a file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			batchCommand(echo, fileName, dbName, cluster)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&echo, "echo", "e", false, "echo batch commands during execution")
	flags.StringVarP(&dbName, "database", "d", defaultDatabase, "name of database to use")
	flags.StringVarP(&fileName, "file", "f", "", "name of file to load")
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")

	return cmd
}
