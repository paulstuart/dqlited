// +build !unit

package main

// provide default server interface, i.e., not running on nginx unit

import (
	"net/http"
)

func ListenAndServe(address string, handler http.Handler) error {
	return http.ListenAndServe(address, handler)
}
