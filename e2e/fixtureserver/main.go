// Command fixtureserver serves the e2e fixtures on a local port, for
// e2e/smoke.bats and for poking at tt without the real API.
//
//	go run ./e2e/fixtureserver -addr 127.0.0.1:4599
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/justinallenmarsh/tangotube-cli/e2e/fixtures"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4599", "where to listen")
	flag.Parse()
	log.Printf("fixture API on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, fixtures.Handler()))
}
