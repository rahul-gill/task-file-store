package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"testing"
)

func TestServerReq(t *testing.T) {
	t.Run("check list files", func(t *testing.T) {

		var client *http.Client
		var remoteURL string
		{
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, err := httputil.DumpRequest(r, true)
				if err != nil {
					panic(err)
				}
				fmt.Printf("%s", b)
			}))
			defer ts.Close()
			client = ts.Client()
			remoteURL = ts.URL
		}
		testCases := []string{"", "wc"}
		for _, testCase := range testCases {
			log.Printf("testing with %s", testCase)
			os.Args = []string{"cmd", testCase}
			CliHandler(client, remoteURL)
			fmt.Printf("\n\n\n")
		}
	})
}
