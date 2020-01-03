// +build unit

package main

// provide optional server interface for running on nginx unit

import (
	"net/http"

	"unit.nginx.org/go"
)

func ListenAndServe(address string, handler http.Handler) error {
	return unit.ListenAndServe(address, handler)
}
