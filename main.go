package main

import (
	//"flag"
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

// DefaultTimeout is the default connection timeout (TODO: verify this assumption)
const DefaultTimeout = time.Second * 60

// will be replaced with version info at link time
var version string = "unknown"

func main() {
	// TODO: these log statements should only print if verbose mode
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
				return fmt.Errorf("invalid log level: %s", level)
			}
			defaultLogLevel = client.LogLevel(run)
			return nil
		},
		TraverseChildren: true,
	}
	cmd.AddCommand(newCluster())
	cmd.AddCommand(newAdhoc())
	cmd.AddCommand(newServer())
	cmd.AddCommand(newDumper())
	cmd.AddCommand(newLoad())
	cmd.AddCommand(newVersion())
	cmd.AddCommand(newHammer())
	cmd.AddCommand(newReport())
	cmd.AddCommand(newTransfer())
	cmd.AddCommand(newAssign())

	flags := cmd.Flags()
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
	var role string
	var id, port int
	var skip bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start a server with web api.",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Println("ENV DIR:", envy.StringDefault("DQLITED_TMP", "/tmp/dqlited"))
			StartServer(id, skip, port, dbName, dir, address, role, timeout, cluster)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&address, "address", "a", envy.StringDefault("DQLITED_ADDRESS", "127.0.0.1:9181"), "address of the node (default is 127.0.0.1:918<ID>)")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.StringVarP(&dir, "dir", "l", envy.StringDefault("DQLITED_TMP", "/tmp/dqlited"), "database working directory")
	flags.StringVarP(&role, "role", "r", envy.StringDefault("DQLITED_ROLE", "voter"), "node role, must be one of: 'voter', 'standby', or 'spare'")
	flags.IntVarP(&id, "id", "i", envy.IntDefault("DQLITED_ID", 1), "server id")
	flags.IntVarP(&port, "port", "p", envy.IntDefault("DQLITED_PORT", 4001), "port to serve traffic on")
	flags.BoolVarP(&skip, "skip", "s", envy.Bool("DQLITED_SKIP"), "do NOT add server to cluster")
	flags.DurationVarP(&timeout, "timeout", "t", time.Minute*5, "time to wait for connection to complete")

	return cmd
}

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

// Return a new transfer command.
func newTransfer() *cobra.Command {
	var cluster []string
	var id uint
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "transfer <id>",
		Short: "transfer leadership to a new node.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Transfer(uint64(id), timeout, cluster)
		},
	}
	flags := cmd.Flags()
	flags.UintVarP(&id, "id", "i", 0, "server id")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for transfer to complete")
	return cmd
}

// Return a new transfer command.
func newAssign() *cobra.Command {
	var cluster []string
	var id uint
	var timeout time.Duration

	roles := map[string]int{
		/*
			Voter.String():     int(Voter),
			Spare{}.String():   int(Spare),
			Standby{}.String(): int(Standby),
		*/
		"voter":   int(Voter),
		"spare":   int(Spare),
		"standby": int(Standby),
	}
	//opts := mapKeys(roles)
	choices := &FlagChoice{choices: mapKeys(roles)}

	cmd := &cobra.Command{
		Use:   "assign <id> <role>",
		Short: "assign a role to a node.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := nodeRole(choices.chosen)
			if err != nil {
				return err
			}
			return Assign(uint64(id), client.NodeRole(r), timeout, cluster)
			//return nil
		},
	}
	flags := cmd.Flags()
	flags.UintVarP(&id, "id", "i", 0, "server id")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for transfer to complete")
	flags.Var(choices, "role", "server role")
	return cmd
}

/*
 */

// Return a new adhoc command.
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

func mapKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

type FlagChoice struct {
	choices []string
	chosen  string
}

func (f *FlagChoice) String() string {
	switch len(f.choices) {
	case 0:
		return "you have no choice"
	case 1:
		return f.choices[0]
	case 2:
		const msg = "%s or %s"
		return fmt.Sprintf(msg, f.choices[0], f.choices[1])
	default:
		var buf strings.Builder
		for i, choice := range f.choices {
			switch i {
			case 0:
				buf.WriteString(choice)
			case len(f.choices) - 1:
				buf.WriteString(", or ")
				buf.WriteString(choice)
			default:
				buf.WriteString(", ")
				buf.WriteString(choice)
			}
		}
		return buf.String()
	}
}

func (f *FlagChoice) Set(value string) error {
	for _, choice := range f.choices {
		if choice == value {
			f.chosen = value
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid choice, must be: %s", value, f.String())
}

func (f *FlagChoice) Type() string {
	return "choose from list"
}
