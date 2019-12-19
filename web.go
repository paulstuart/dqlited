package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type WebHandler struct {
	Path string
	Func http.HandlerFunc
}

/*
// RaftResponse is the Raft metadata that will be included with responses, if
// the associated request modified the Raft log.
type RaftResponse struct {
	Index  uint64 `json:"index,omitempty"`
	NodeID string `json:"node_id,omitempty"`
}
*/

// Response represents a response from the HTTP service.
type Response struct {
	Results interface{}   `json:"results,omitempty"`
	Error   string        `json:"error,omitempty"`
	Time    float64       `json:"time,omitempty"`
//	Raft    *RaftResponse `json:"raft,omitempty"` // TODO: remove this after making pydqlite working
}

/*
"/db/backup",
"/db/connections",
"/db/execute",
"/db/load",
"/db/query",
*/

func myIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		os.Stderr.WriteString("Oops: " + err.Error() + "\n")
		os.Exit(1)
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	panic("no IP address for you!")
}

// WebServer provides a web interface to the database
func WebServer(port int, dbname string, cluster []string) {
	handlers, err := setupHandlers(dbname, cluster)
	if err != nil {
		panic(err)
	}
	webServer(port, handlers...)
}

func webServer(port int, handlers ...WebHandler) {
	for _, handler := range handlers {
		http.Handle(handler.Path, handler.Func)
	}
	//http.HandleFunc("/favicon.ico", faviconPage)

	httpServer := fmt.Sprintf(":%d", port)
	fmt.Printf("serve up web: http://%s%s/\n", myIP(), httpServer)
	err := http.ListenAndServe(httpServer, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func setupHandlers(dbname string, cluster []string) ([]WebHandler, error) {
	x, err := NewConnection(dbname, cluster)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make queryer")
	}
	return makeHandlers(x, x), nil
}

func makeHandlers(exec Executor, query Queryor) []WebHandler {
	return []WebHandler{
		{"/db/execute", makeHandleExec(exec)},
		{"/db/query", makeHandleQuery(query)},
		{"/", homePage},
	}
}

func homePage(w http.ResponseWriter, r *http.Request) {
	log.Println("home page has been hit")
	w.Write([]byte("nothing to see here"))
}

func makeHandleExec(exec Executor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("exec page has been hit")
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var statements []string
		if err := json.NewDecoder(r.Body).Decode(&statements); err != nil {
			log.Printf("exec error getting queries: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp, err := exec.Execute(statements...)
		if err != nil {
			log.Printf("error executing queries: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeResponse(w, r, resp)
	}
}

func makeHandleQuery(queryor Queryor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("query page has been hit")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// TODO: add perm check and timing
		if r.Method != "GET" && r.Method != "POST" {
			log.Printf("invalid method: %q\n", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		queries, err := requestQueries(r)
		if err != nil {
			log.Printf("error getting queries: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("queries submitted: %v\n", queries)
		// TODO: undo the mutiple queries rqlite nonsense
		resp, err := queryor.QueryDB(queries[0])
		if err != nil {
			log.Printf("error executing queries: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		reply := Response{
			Results: resp,
		}
		enc := json.NewEncoder(w)
		enc.Encode(reply)
	}
}

func requestQueries(r *http.Request) ([]string, error) {
	if r.Method == "GET" {
		query, err := stmtParam(r)
		if err != nil || query == "" {
			return nil, errors.New("bad query GET request")
		}
		return []string{query}, nil
	}

	qs := []string{}
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		return nil, errors.New("bad query POST request")
	}
	if err := json.Unmarshal(b, &qs); err != nil {
		return nil, errors.New("bad query POST request")
	}
	if len(qs) == 0 {
		return nil, errors.New("bad query POST request")
	}

	return qs, nil
}

// queryParam returns whether the given query param is set to true.
func queryParam(req *http.Request, param string) (bool, error) {
	err := req.ParseForm()
	if err != nil {
		return false, err
	}
	if _, ok := req.Form[param]; ok {
		return true, nil
	}
	return false, nil
}

// durationParam returns the duration of the given query param, if set.
func durationParam(req *http.Request, param string) (time.Duration, bool, error) {
	q := req.URL.Query()
	t := strings.TrimSpace(q.Get(param))
	if t == "" {
		return 0, false, nil
	}
	dur, err := time.ParseDuration(t)
	if err != nil {
		return 0, false, err
	}
	return dur, true, nil
}

// stmtParam returns the value for URL param 'q', if present.
func stmtParam(req *http.Request) (string, error) {
	q := req.URL.Query()
	return strings.TrimSpace(q.Get("q")), nil
}

// fmtParam returns the value for URL param 'fmt', if present.
func fmtParam(req *http.Request) (string, error) {
	q := req.URL.Query()
	return strings.TrimSpace(q.Get("fmt")), nil
}

// isPretty returns whether the HTTP response body should be pretty-printed.
func isPretty(req *http.Request) (bool, error) {
	return queryParam(req, "pretty")
}

// isAtomic returns whether the HTTP request is an atomic request.
func isAtomic(req *http.Request) (bool, error) {
	// "transaction" is checked for backwards compatibility with
	// client libraries.
	for _, q := range []string{"atomic", "transaction"} {
		if a, err := queryParam(req, q); err != nil || a {
			return a, err
		}
	}
	return false, nil
}

// noLeader returns whether processing should skip the leader check.
func noLeader(req *http.Request) (bool, error) {
	return queryParam(req, "noleader")
}

// timings returns whether timings are requested.
func timings(req *http.Request) (bool, error) {
	return queryParam(req, "timings")
}

// txTimeout returns the duration of any transaction timeout set.
func txTimeout(req *http.Request) (time.Duration, bool, error) {
	return durationParam(req, "tx_timeout")
}

// idleTimeout returns the duration of any idle connection timeout set.
func idleTimeout(req *http.Request) (time.Duration, bool, error) {
	return durationParam(req, "idle_timeout")
}

/*
// level returns the requested consistency level for a query
func level(req *http.Request) (store.ConsistencyLevel, error) {
	q := req.URL.Query()
	lvl := strings.TrimSpace(q.Get("level"))

	switch strings.ToLower(lvl) {
	case "none":
		return store.None, nil
	case "weak":
		return store.Weak, nil
	case "strong":
		return store.Strong, nil
	default:
		return store.Weak, nil
	}
}
*/

// backuFormat returns the request backup format, setting the response header
// accordingly.
/*
func backupFormat(w http.ResponseWriter, r *http.Request) (store.BackupFormat, error) {
	fmt, err := fmtParam(r)
	if err != nil {
		return store.BackupBinary, err
	}
	if fmt == "sql" {
		w.Header().Set("Content-Type", "application/sql")
		return store.BackupSQL, nil
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	return store.BackupBinary, nil
}
*/

func prettyEnabled(e bool) string {
	if e {
		return "enabled"
	}
	return "disabled"
}

// NormalizeAddr ensures that the given URL has a HTTP protocol prefix.
// If none is supplied, it prefixes the URL with "http://".
func NormalizeAddr(addr string) string {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return fmt.Sprintf("http://%s", addr)
	}
	return addr
}
