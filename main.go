package main

import (
	//"flag"
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/go-dqlite/client"
	"github.com/paulstuart/envy"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const defaultDatabase = "demo.db"

// DefaultTimeout is the default connection timeout (TODO: verify this assumption)
const DefaultTimeout = time.Second * 60

// will be replaced with version info at link time
var version string = "unknown"

var verbose bool

func main() {
	// TODO: these log statements should only print if verbose mode
	//log.Println("starting dqlited with PID:", os.Getpid())
	//defer log.Println("exiting dqlited with PID:", os.Getpid())

	cmd := path.Base(os.Args[0])
	root := newRoot(cmd)
	if err := root.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// Return a new root command.
func newRoot(cmdName string) *cobra.Command {
	var level, logfile string
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
			if logfile != "" {
				f, err := os.Create(logfile)
				if err != nil {
					return errors.Wrapf(err, "error opening log file: %s", logfile)
				}
				log.SetOutput(f)
			}
			return nil
		},
		TraverseChildren: true,
	}
	cmd.AddCommand(newStatus())
	cmd.AddCommand(newAdhoc())
	cmd.AddCommand(newAddnode())
	cmd.AddCommand(newServer())
	cmd.AddCommand(newDumper())
	cmd.AddCommand(newLoad())
	cmd.AddCommand(newVersion())
	cmd.AddCommand(newHammer())
	cmd.AddCommand(newReport())
	cmd.AddCommand(newTransfer())
	cmd.AddCommand(newAssign())
	cmd.AddCommand(newRemove())
	cmd.AddCommand(newLeaderID())

	flags := cmd.Flags()
	flags.StringVarP(&level, "level", "z", "error", "log level (debug, info, warn, error)")
	flags.StringVarP(&logfile, "out", "o", "", "log to file (default is stderr")
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
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			//func StartServer(ctx context.Context, id int, dir, address, web string, cluster []string) error {
			//err := StartServer(ctx, id, port, skip, dbName, dir, address, role, cluster)
			cluster = omit(address, cluster)
			err := StartServer(ctx, id, port, dir, address, cluster)
			log.Println("server is done serving:", err)
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

// Return a status command.
func newStatus() *cobra.Command {
	var cluster []string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "status",
		Short: "display cluster nodes.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			return clusterShow(ctx, cluster...)
		},
	}
	flags := cmd.Flags()
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for connection to complete")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")

	return cmd
}

// Return a leader id command.
func newLeaderID() *cobra.Command {
	var cluster []string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "leader",
		Short: "print the id of the current leader node.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			id, err := LeaderID(ctx, cluster)
			fmt.Println(id)
			return err
		},
	}
	flags := cmd.Flags()
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for connection to complete")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")

	return cmd
}

// Return a new transfer command.
func newTransfer() *cobra.Command {
	var cluster []string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "transfer <id>",
		Short: "transfer leadership to a new node.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if err := Transfer(ctx, id, cluster); err != nil {
				fmt.Printf("transfer to node: %d failed: %v\n", id, err)
				os.Exit(1)
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for transfer to complete")
	return cmd
}

// Return a new remove command.
func newRemove() *cobra.Command {
	var cluster []string
	var id uint
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "remove -i <id>",
		Short: "remove a node.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			return Remove(ctx, uint64(id), cluster)
		},
	}
	flags := cmd.Flags()
	flags.UintVarP(&id, "id", "i", 0, "server id")
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for transfer to complete")
	return cmd
}

// Add a node to the cluster
func newAddnode() *cobra.Command {
	var cluster []string
	var address string
	//var role string
	var timeout time.Duration

	roles := map[string]int{
		client.Voter.String():   int(Voter),
		client.Spare.String():   int(Spare),
		client.StandBy.String(): int(Standby),
	}
	choices := &FlagChoice{choices: mapKeys(roles), chosen: client.Voter.String()}

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a node to the cluster.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				log.Fatalln(err)
			}
			role, err := nodeRole(choices.chosen)
			if err != nil {
				log.Fatalln(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			client, err := getLeader(ctx, cluster)
			if err != nil {
				log.Fatalln(err)
			}

			if err := nodeAdd(ctx, client, id, role, address); err != nil {
				log.Fatalln("error adding node:", err)
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&address, "address", "a", envy.StringDefault("DQLITED_ADDRESS", "127.0.0.1:9181"), "address of the node (default is 127.0.0.1:918<ID>)")
	flags.VarP(choices, "role", "r", "server role")
	flags.DurationVarP(&timeout, "timeout", "t", time.Minute*5, "time to wait for connection to complete")

	return cmd
}

// Return a new assign command.
func newAssign() *cobra.Command {
	var cluster []string
	var timeout time.Duration

	roles := map[string]int{
		client.Voter.String():   int(Voter),
		client.Spare.String():   int(Spare),
		client.StandBy.String(): int(Standby),
	}
	choices := &FlagChoice{choices: mapKeys(roles), chosen: client.Voter.String()}

	cmd := &cobra.Command{
		Use:   "assign <id>",
		Short: "assign a role to a node (voter, spare, or standby).",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := nodeRole(choices.chosen)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			for _, arg := range args {
				id, err := strconv.ParseUint(arg, 10, 64)
				if err != nil {
					log.Fatalln(err)
				}
				if err := Assign(ctx, uint64(id), client.NodeRole(r), cluster); err != nil {
					log.Fatalln(err)
				}
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.DurationVarP(&timeout, "timeout", "t", time.Second*60, "time to wait for transfer to complete")
	flags.VarP(choices, "role", "r", "server role")
	return cmd
}

// Return a new adhoc command.
func newAdhoc() *cobra.Command {
	var cluster []string
	var dbName string
	var divs, headless bool

	cmd := &cobra.Command{
		Use:   "adhoc <statment>...",
		Short: "execute a statement against the database.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			logger := NewLogFunc(defaultLogLevel, "adhoc:: ", NewLoggingWriter())
			err := dbCmd(ctx, dbName, cluster, logger, !headless, divs, strings.Join(args, " "))
			if err != nil {
				cause := errors.Cause(err)
				if sqlErr, ok := cause.(SqliteError); ok {
					code, msg := sqlErr.SqliteError()
					fmt.Printf("SqliteError (%d): %s\n", code, msg)
				} else {
					fmt.Printf("(%T): %v\n", cause, err)
				}
				os.Exit(1)
			}
			return nil
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
		Short: "dump the database (and its associated WAL file).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			return dbDump(ctx, dbName, cluster)
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
			ctx := context.Background()
			dbFile(ctx, fileName, dbName, batch, verbose, cluster)
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
		Use:   "query",
		Short: "Execute the queries in in the given file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			dbReport(ctx, fileName, dbName, headers, lines, cluster)
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

// run a load test against the database
func newHammer() *cobra.Command {
	var cluster []string
	var dbName string
	var id int
	var count int

	cmd := &cobra.Command{
		Use:   "hammer",
		Short: "load test the database.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("defaultLogLevel:", defaultLogLevel)
			logger := NewLogFunc(defaultLogLevel, "hammy:: ", log.Writer())
			logger(LogDebug, "checking hammer logger")
			hammer(id, count, logger, dbName, cluster...)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&cluster, "cluster", "c", clusterList(), "addresses of existing cluster nodes")
	flags.StringVarP(&dbName, "database", "d", envy.StringDefault("DQLITED_DB", defaultDatabase), "name of database to use")
	flags.IntVarP(&count, "count", "n", 10000, "how many times to repeat")
	flags.IntVarP(&id, "id", "i", envy.IntDefault("DQLITED_ID", 1), "server id")

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

// FlagChoice is a f FlagSet that allows the user to pick from a list
type FlagChoice struct {
	choices []string
	chosen  string
}

// NewFlagChoice returns a new FlagChoice
func NewFlagChoice(choices []string, chosen string) *FlagChoice {
	return &FlagChoice{
		choices: choices,
		chosen:  chosen,
	}
}

// String satisfies the FlagSet interface
func (f *FlagChoice) String() string {
	return choiceList(f.choices...)
}

// create a nice comma-separated list with the last option using 'or' to separate
func choiceList(choices ...string) string {
	switch len(choices) {
	case 0:
		return "you have no choice"
	case 1:
		return choices[0]
	case 2:
		const msg = "%s or %s"
		return fmt.Sprintf(msg, choices[0], choices[1])
	default:
		var buf strings.Builder
		for i, choice := range choices {
			switch i {
			case 0:
				buf.WriteString(choice)
			case len(choices) - 1:
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

// Set satisfies the FlagSet interface
func (f *FlagChoice) Set(value string) error {
	for _, choice := range f.choices {
		if choice == value {
			f.chosen = value
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid choice, must be: %s", value, f.String())
}

// Type satisfies the FlagSet interface
func (f *FlagChoice) Type() string {
	return choiceList(f.choices...)
}
