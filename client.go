package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	app "github.com/canonical/go-dqlite/app"
	"github.com/canonical/go-dqlite/client"
)

var defaultLogLevel = client.LogError

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

// NewLogFunc returns a logging function used by dqlite
func NewLogFunc(level client.LogLevel, prefix string, w io.Writer) client.LogFunc {
	if w == nil {
		w = os.Stdout
	}
	//log.Println("making NewLogger with level:", level)
	return func(l client.LogLevel, format string, a ...interface{}) {
		// log levels start at 0 for Debug and increase up to Error
		// only print logs within that limit
		if l >= level {
			log.Printf(prefix+format, a...)
		}
	}
}

// DefaultLogger returns a logger using the default settings
func DefaultLogger(w io.Writer) client.LogFunc {
	return NewLogFunc(defaultLogLevel, "", w)
	//return client.NewLogFunc(defaultLogLevel, "", w)
}

type logWriter struct{}

func (l *logWriter) Write(in []byte) (int, error) {
	log.Println(string(in))
	return len(in), nil
}

// NewLoggingWriter returns an io.Writer using the default Go logger
func NewLoggingWriter() io.Writer {
	//return NewLogFunc(defaultLogLevel, "", w)
	return &logWriter{}
}

// NewLogLog returns a logging function used by dqlite
func NewLogLog(level client.LogLevel) client.LogFunc {
	return NewLogFunc(level, "", NewLoggingWriter())
}

func getLeader(ctx context.Context, cluster []string) (*client.Client, error) {
	store := getStore(ctx, cluster)
	//return client.FindLeader(ctx, store, client.WithLogFunc(NewLogger(defaultLogLevel, log.Writer())))
	//logFunc := client.NewLogFunc(defaultLogLevel, "", log.Writer())
	logFunc := NewLogFunc(defaultLogLevel, "", nil)
	//return client.FindLeader(ctx, store, client.WithLogFunc(client.NewLogFunc(defaultLogLevel, "", log.Writer())))

	dial := client.DefaultDialFunc
	/*
				crt := "cert.pem"
				key := "key.pem"
			crt := "certs/server.pem"
			key := "certs/server.key"
		crt := "server.crt"
		key := "server.key"
	*/
	crt := "cluster.crt"
	key := "cluster.key"
	log.Println("get leader")
	if crt != "" {
		cert, err := tls.LoadX509KeyPair(crt, key)
		if err != nil {
			return nil, err
		}
		/*
		 */
		data, err := ioutil.ReadFile(crt)
		if err != nil {
			return nil, err
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("bad certificate")
		}

		config := app.SimpleDialTLSConfig(cert, pool)
		dial = client.DialFuncWithTLS(dial, config)
		log.Println("using TLS encryption")

		//listen, dial := app.SimpleTLSConfig(cert, pool)
		/*
			listen := app.SimpleListenTLSConfig(cert, pool)
		*/
		//options = append(options, app.WithTLS(listen, dial))
	}

	return client.FindLeader(ctx, store, client.WithLogFunc(logFunc), client.WithDialFunc(dial))
}

func getStore(ctx context.Context, cluster []string) client.NodeStore {
	//log.Println("GET STORE FOR CLUSTER:", cluster)
	store := client.NewInmemNodeStore()
	if len(cluster) == 0 {
		cluster = defaultCluster
	}
	infos := make([]client.NodeInfo, len(cluster))
	for i, address := range cluster {
		infos[i].ID = uint64(i + 1)
		infos[i].Address = address
	}
	//log.Println("INFOS:", infos)
	store.Set(ctx, infos)
	return store
}
