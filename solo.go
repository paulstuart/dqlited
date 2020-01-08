// +build !unit

package main

// provide default server interface, i.e., not running on nginx unit

import (
	"net/http"
)

// ListenAndServe abstracts http serving (to allow replacement
// with nginx Unit
func ListenAndServe(address string, handler http.Handler) error {
	return http.ListenAndServe(address, handler)
}
