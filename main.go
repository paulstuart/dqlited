package main

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const defaultDatabase = "demo.db"

var version string = "unknown"

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
	cmd.AddCommand(newServer())
	cmd.AddCommand(newDumper())
	cmd.AddCommand(newLoad())
	cmd.AddCommand(newSeeker())
	cmd.AddCommand(newVersion())

	return cmd
}

// Start a web server for remote clients.
func newServer() *cobra.Command {
	var cluster []string
	var dir string
	var address string
	var dbName string
	var id, port int
	var skip bool

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start a server with web api.",
		RunE: func(cmd *cobra.Command, args []string) error {
			StartServer(id, skip, port, dbName, dir, address, cluster)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&dbName, "database", "d", defaultDatabase, "name of database to use")
	flags.StringVarP(&dir, "dir", "l", "/tmp/dqlited", "database working directory")
	flags.StringVarP(&address, "address", "a", "", "address of the node (default is 127.0.0.1:918<ID>)")
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")
	flags.IntVarP(&id, "id", "i", 1, "server id")
	flags.IntVarP(&port, "port", "p", 4001, "port to serve traffic on")
	flags.BoolVarP(&skip, "skip", "s", false, "do NOT add server to cluster")

	return cmd
}

// Return a new start command.
func newStart() *cobra.Command {
	var dir string
	var address string
	var dbName string
	var cluster []string

	cmd := &cobra.Command{
		Use:   "start <id>",
		Short: "Start a dqlite node.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrapf(err, "%s is not a number", args[0])
			}
			return dbStart(id, dir, address, dbName, cluster...)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&dir, "dir", "l", "/tmp/dqlited", "database working directory")
	flags.StringVarP(&dbName, "database", "d", defaultDatabase, "name of database to use")
	flags.StringVarP(&address, "address", "a", "", "address of the node (default is 127.0.0.1:918<ID>)")
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")

	return cmd
}

// Return a cluster nodes command.
func newCluster() *cobra.Command {
	var address string
	var cluster []string

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "display cluster nodes.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return clusterShow(address, cluster...)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&address, "address", "a", "", "address of the node to add (default is 127.0.0.1:918<ID>)")
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")
	return cmd
}

// Return a new add command.
func newAdd() *cobra.Command {
	var address string
	var cluster []string

	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add a node to the dqlite-demo cluster.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrapf(err, "%s is not a number", args[0])
			}
			return clusterAdd(id, address, cluster)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&address, "address", "a", "", "address of the node to add (default is 127.0.0.1:918<ID>)")
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")

	return cmd
}

// Return a new update key command.
func newAdhoc() *cobra.Command {
	var cluster []string
	var dbName string
	var divs bool

	cmd := &cobra.Command{
		Use:   "adhoc <statment>...",
		Short: "execute a statement against the demo database.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dbCmd(dbName, cluster, divs, strings.Join(args, " "))
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&divs, "dividers", "l", false, "print lines between columns")
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

// load a file containing sql statements
func newLoad() *cobra.Command {
	var cluster []string
	var dbName string
	var fileName string

	cmd := &cobra.Command{
		Use:   "load",
		Short: "Execute the statements in in the given file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbFile(fileName, dbName, cluster)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&dbName, "database", "d", defaultDatabase, "name of database to use")
	flags.StringVarP(&fileName, "file", "f", "", "name of file to load")
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")

	return cmd
}

// execute a poller that shows current leader
func newSeeker() *cobra.Command {
	var cluster []string

	cmd := &cobra.Command{
		Use:   "seeker",
		Short: "Continuously poll for active leader.",
		RunE: func(cmd *cobra.Command, args []string) error {
			seeker(cluster...)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", defaultCluster, "addresses of existing cluster nodes")

	return cmd
}

// execute a poller that shows current leader
func newVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "show build version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(version)
			return nil
		},
	}

	return cmd
}
