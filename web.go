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
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const assetsDir = "assets"

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

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Llongfile)
}

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

// StarServer provides a web interface to the database
// No error to return as it's never intended to stop
func StartServer(id int, skip bool, port int, dbname, dir, address string, timeout time.Duration, cluster []string) {
	log.Printf("starting server node :%d dir: %q ip:%s\n", id, dir, myIP())
	if err := nodeStart(id, !skip, dir, address, timeout, cluster...); err != nil {
		panic(err)
	}
	log.Printf("setting up handlers for database: %s\n", dbname)
	handlers, err := setupHandlers(dbname, cluster)
	if err != nil {
		panic(err)
	}
	log.Printf("starting webserver port: %d\n", port)
	webServer(port, handlers...)
}

func faviconPage() http.HandlerFunc {
	favicon := path.Join(assetsDir, "favicon.ico")
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, favicon)
	}
}

func webServer(port int, handlers ...WebHandler) {
	for _, handler := range handlers {
		http.Handle(handler.Path, handler.Func)
	}
	http.HandleFunc("/favicon.ico", faviconPage())

	httpServer := fmt.Sprintf(":%d", port)
	fmt.Printf("serve up web: http://%s%s/\n", myIP(), httpServer)
	err := ListenAndServe(httpServer, nil)
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
	w.Write([]byte("nothing to see here\n"))
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

func makeHandleExec(exec Executor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		for i, query := range queries {
			resp, err := queryor.QueryRows(query)
			if err != nil {
				log.Printf("error executing queries (%d/%d): %q %v\n", i+1, len(queries), query, err)
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

/*
func prettyEnabled(e bool) string {
	if e {
		return "enabled"
	}
	return "disabled"
}
*/

// NormalizeAddr ensures that the given URL has a HTTP protocol prefix.
// If none is supplied, it prefixes the URL with "http://".
func NormalizeAddr(addr string) string {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return fmt.Sprintf("http://%s", addr)
	}
	return addr
}
