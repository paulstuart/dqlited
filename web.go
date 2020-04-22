package main

import (
	//"context"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"path"
	"strings"
	"time"

	"github.com/canonical/go-dqlite/app"
	"github.com/pkg/errors"
)

const assetsDir = "assets"

// WebHandler is the mapping of a path to its http handler
type WebHandler struct {
	Path string
	Func http.HandlerFunc
}

// Response represents a response from the HTTP service.
type Response struct {
	Results interface{} `json:"results,omitempty"`
	Error   string      `json:"error,omitempty"`
	Time    float64     `json:"time,omitempty"`
}

func myIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
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

func faviconPage() http.HandlerFunc {
	favicon := path.Join(assetsDir, "favicon.ico")
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, favicon)
	}
}

func ServerHandlers(port int, handlers ...WebHandler) {
	for _, handler := range handlers {
		http.Handle(handler.Path, handler.Func)
	}
	http.HandleFunc("/favicon.ico", faviconPage())

	httpServer := fmt.Sprintf(":%d", port)
	log.Printf("serve up web: http://%s%s/\n", myIP(), httpServer)
	err := ListenAndServe(httpServer, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func webServer(port int, handlers ...WebHandler) {
	for _, handler := range handlers {
		http.Handle(handler.Path, handler.Func)
	}

	httpServer := fmt.Sprintf(":%d", port)
	log.Printf("serve up web: http://%s%s/\n", myIP(), httpServer)
	err := ListenAndServe(httpServer, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func webHandlers(ctx context.Context, dq *app.App) []WebHandler {
	return []WebHandler{
		{"/db/execute/", makeHandleExec(ctx, dq)},
		{"/db/query/", makeHandleQuery(ctx, dq)},
		{"/status", makeHandleStatus(dq)},
		{"/favicon.ico", faviconPage()},
		{"/", homePage},
	}
}

func homePage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("nothing to see here\n"))
}

func statusPage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("nothing to see here\n"))
}

type nodeStatus struct {
	ID      uint64
	Address string
	Role    string
}

func makeHandleStatus(dq *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		leader, err := dq.Leader(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		nodes, err := leader.Cluster(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		status := make([]nodeStatus, len(nodes))
		for i, node := range nodes {
			status[i] = nodeStatus{
				node.ID,
				node.Address,
				node.Role.String(),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(status)
	}
}
func writeResponse(w http.ResponseWriter, r *http.Request, j *ExecuteResponse) {
	enc := json.NewEncoder(w)
	if pretty, _ := isPretty(r); pretty {
		enc.SetIndent("", "    ")
	}

	if err := enc.Encode(j); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

//func makeHandleExec(exec Executor) http.HandlerFunc {
//func makeHandleExec(ctx context.Context, dq *app.App, dbname string) http.HandlerFunc {
func makeHandleExec(ctx context.Context, dq *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		dbname := r.URL.Path
		if i := strings.LastIndex(dbname, "/"); i > 0 {
			dbname = dbname[i+1:]
		}

		// TODO: sort out who goes first
		ctx = r.Context()
		if dead, ok := ctx.Deadline(); ok {
			//ctx = r.Context().WithTimeout(ctx) // inherit timeout. invert it?
			dead = dead.Add(time.Minute)
			fmt.Println("DBNAME:", dbname, "TIME REMAINING:", dead.Sub(time.Now()))
		}
		db, err := dq.Open(ctx, dbname)
		if err != nil {
			log.Printf("error opening db: %q -- %v\n", dbname, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			//w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer db.Close()

		log.Println("OPENED DB:", dbname)
		defer r.Body.Close()
		var statements []string
		if err := json.NewDecoder(r.Body).Decode(&statements); err != nil {
			log.Printf("exec error getting queries: %v\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx2 := r.Context()
		if dead, ok := ctx.Deadline(); ok {
			//ctx = r.Context().WithTimeout(ctx) // inherit timeout. invert it?
			dead = dead.Add(time.Minute)
			fmt.Println("TIME REMAINING:", dead.Sub(time.Now()))
			ctx2, _ = context.WithDeadline(ctx2, dead) // inherit timeout. invert it?
		}
		stmts := strings.Join(statements, "\n")
		resp, err := ExecuteContext(ctx2, db, stmts)
		if err != nil {
			log.Printf("error executing queries: %v\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			//w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeResponse(w, r, resp)
	}
}

//func makeHandleQuery(queryor Queryor) http.HandlerFunc {
func makeHandleQuery(ctx context.Context, dq *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "POST" {
			log.Printf("invalid method: %q\n", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		dbname := r.URL.Path
		if i := strings.LastIndex(dbname, "/"); i > 0 {
			dbname = dbname[i+1:]
		}

		// TODO: sort out merging context cancelation/timeout
		ctx = r.Context()
		db, err := dq.Open(ctx, dbname)
		if err != nil {
			log.Printf("error opening db: %q -- %v\n", dbname, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer db.Close()
		log.Println("OPENED DB:", dbname)

		queries, err := requestQueries(r)
		if err != nil {
			log.Printf("error getting queries: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("queries submitted: %v\n", queries)

		for i, query := range queries {
			ctx2 := r.Context()
			if dead, ok := ctx.Deadline(); ok {
				dead = dead.Add(time.Minute)
				fmt.Println("TIME REMAINING:", dead.Sub(time.Now()))

				ctx3, cancel := context.WithDeadline(ctx, dead) // inherit timeout. invert it?
				defer cancel()
				ctx2 = ctx3

			}
			resp, err := QueryRows(ctx2, db, query)
			//resp, err := queryor.QueryRows(query.)
			if err != nil {
				log.Printf("error executing queries (%d/%d): %q %v\n", i+1, len(queries), query, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			reply := Response{
				Results: resp,
			}
			enc := json.NewEncoder(w)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			enc.Encode(reply)
		}
	}
}

// return the db queries submitted with the request
func requestQueries(r *http.Request) ([]string, error) {
	if r.Method == "GET" {
		query, err := stmtParam(r)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get statement parameters")
		}
		if query == "" {
			return nil, errors.New("no query given")
		}
		return []string{query}, nil
	}

	defer r.Body.Close()

	qs := []string{}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed reading request body")
	}
	if err := json.Unmarshal(b, &qs); err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling request")
	}
	if len(qs) == 0 {
		return nil, errors.New("empty request")
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
// TODO: apply or remove this functionality (used by pydqlite)
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

// NormalizeAddr ensures that the given URL has a HTTP protocol prefix.
// If none is supplied, it prefixes the URL with "http://".
func NormalizeAddr(addr string) string {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return fmt.Sprintf("http://%s", addr)
	}
	return addr
}
