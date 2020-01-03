package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/canonical/go-dqlite/client"
	"github.com/paulstuart/envy"
	"github.com/spf13/cobra"
)

const defaultDatabase = "demo.db"
const DefaultTimeout = time.Second * 60

// will be replaced with version info at link time
var version string = "unknown"

func main() {
	log.Println("starting dqlited with PID:", os.Getpid())
	defer log.Println("exiting dqlited with PID:", os.Getpid())
	cmd := path.Base(os.Args[0])
	root := newRoot(cmd)
	if err := root.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// Return a new root command.
func newRoot(cmdName string) *cobra.Command {
	//var choice map[string]int
	var level string

	opts := map[string]int{
		"debug": int(client.LogDebug),
		"info":  int(client.LogInfo),
		"warn":  int(client.LogWarn),
		"error": int(client.LogError),
	}

	cmd := &cobra.Command{
		Use:   cmdName,
		Short: "Manage dqlite servers",
		/*
			RunE: func(cmd *cobra.Command, args []string) error {
				//return fmt.Errorf("not implemented")
				return nil
			},
		*/
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			run, ok := opts[level]
			if !ok {
				return fmt.Errorf("invalid log level: %s\n", level)
			}
			defaultLogLevel = client.LogLevel(run)
			return nil
		},
		TraverseChildren: true,
	}
	//cmd.AddCommand(newStart())
	//cmd.AddCommand(newAdd())
	cmd.AddCommand(newCluster())
	cmd.AddCommand(newAdhoc())
	cmd.AddCommand(newServer())
	cmd.AddCommand(newDumper())
	cmd.AddCommand(newLoad())
	//cmd.AddCommand(newSeeker())
	cmd.AddCommand(newVersion())
	cmd.AddCommand(newHammer())
	cmd.AddCommand(newReport())

	flags := cmd.Flags()
	/*
		have (*map[string]int, string, map[string]int, string)
		want (*map[string]int, string, string, map[string]int, string)

	*/
	//flags.StringToIntVarP(&choice, "level", "y", opts, "set logging level")
	flags.StringVarP(&level, "level", "z", "error", "log level (debug, info, warn, error)")
	return cmd
}

// return the "default" clusters
func clusterList() []string {
	if c := envy.String("DQLITED_CLUSTER"); c != "" {
		return strings.Split(c, ",")
	}
	return defaultCluster
}

// Start a web server for remote clients.
func newServer() *cobra.Command {
	var cluster []string
	var dir string
	var address string
	var dbName string
	var id, port int
	var skip bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start a server with web api.",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Println("ENV DIR:", envy.StringDefault("DQLITED_TMP", "/tmp/dqlited"))
			StartServer(id, skip, port, dbName, dir, address, timeout, cluster)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&address, "address", "a", envy.StringDefault("DQLITED_ADDRESS", "127.0.0.1:9181"), "address of the node (default is 127.0.0.1:918<ID>)")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.StringVarP(&dir, "dir", "l", envy.StringDefault("DQLITED_TMP", "/tmp/dqlited"), "database working directory")
	flags.IntVarP(&id, "id", "i", envy.IntDefault("DQLITED_ID", 1), "server id")
	flags.IntVarP(&port, "port", "p", envy.IntDefault("DQLITED_PORT", 4001), "port to serve traffic on")
	flags.BoolVarP(&skip, "skip", "s", envy.Bool("DQLITED_SKIP"), "do NOT add server to cluster")
	flags.DurationVarP(&timeout, "timeout", "t", time.Minute*5, "time to wait for connection to complete")

	return cmd
}

/*
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
	flags.StringVarP(&address, "address", "a", envy.StringDefault("DQLITED_ADDRESS", "127.0.0.1:9181"), "address of the node (default is 127.0.0.1:918<ID>)")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.StringVarP(&dir, "dir", "l", envy.StringDefault("DQLITED_TMP", "/tmp/dqlited"), "database working directory")

	return cmd
}
*/

// Return a cluster nodes command.
func newCluster() *cobra.Command {
	var address string
	var cluster []string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "display cluster nodes.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return clusterShow(address, timeout, cluster...)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&address, "address", "a", envy.StringDefault("DQLITED_ADDRESS", "127.0.0.1:9181"), "address of the node (default is 127.0.0.1:918<ID>)")
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for connection to complete")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")

	return cmd
}

// Return a new add command.
/*
func newAdd() *cobra.Command {
	var address string
	var cluster []string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add a node to the dqlite-demo cluster.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrapf(err, "%s is not a number", args[0])
			}
			return clusterAdd(id, address, timeout, cluster)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&address, "address", "a", envy.StringDefault("DQLITED_ADDRESS", "127.0.0.1:9181"), "address of the node (default is 127.0.0.1:918<ID>)")
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for connection to complete")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")

	return cmd
}
*/

// Return a new update key command.
func newAdhoc() *cobra.Command {
	var cluster []string
	var dbName string
	var divs, headless bool

	cmd := &cobra.Command{
		Use:   "adhoc <statment>...",
		Short: "execute a statement against the demo database.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dbCmd(dbName, cluster, !headless, divs, strings.Join(args, " "))
		},
	}
	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.BoolVarP(&divs, "dividers", "l", false, "print lines between columns")
	flags.BoolVarP(&headless, "no-header", "t", false, "don't print table header")
	return cmd
}

// Return a new database dump command.
func newDumper() *cobra.Command {
	var cluster []string
	var dbName string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "dump database",
		Short: "dump the database and its associated WAL file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return dbDump(dbName, timeout, cluster)
		},
	}
	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for connection to complete")

	return cmd
}

// load a file containing sql statements
func newLoad() *cobra.Command {
	var cluster []string
	var dbName string
	var fileName string
	var batch, verbose bool

	cmd := &cobra.Command{
		Use:   "load",
		Short: "Execute the statements in the given file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fileName == "" {
				log.Fatal("no filename specified")
			}
			dbFile(fileName, dbName, batch, verbose, cluster)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.StringVarP(&fileName, "file", "f", "", "name of file to load")
	flags.BoolVarP(&batch, "batch", "b", false, "run all statements as a single transaction")
	flags.BoolVarP(&verbose, "verbose", "v", false, "be chatty about activities")

	return cmd
}

// report a file containing multiple sql query statements
func newReport() *cobra.Command {
	var cluster []string
	var dbName string
	var fileName string
	var headers, lines bool

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Execute the queries in in the given file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbReport(fileName, dbName, headers, lines, cluster)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.StringVarP(&fileName, "file", "f", "", "name of file to load")
	flags.BoolVarP(&headers, "headers", "b", true, "show table headings")
	flags.BoolVarP(&lines, "lines", "v", false, "print lines between columns")

	return cmd
}

/*
// execute a poller that shows current leader
func newSeeker() *cobra.Command {
	var cluster []string
	var dbName string
	var query string
	var pause time.Duration

	cmd := &cobra.Command{
		Use:   "seeker",
		Short: "Continuously query the database.",
		RunE: func(cmd *cobra.Command, args []string) error {
			seeker(dbName, query, pause, cluster...)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.StringVarP(&query, "query", "q", "select count(*) from sqlite_master;", "query to execute")
	flags.DurationVarP(&pause, "pause", "p", time.Second, "time to pause between queries")

	return cmd
}
*/

// show the application version and exit
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
