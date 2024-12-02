package main

import (
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerResp(t *testing.T) {
	t.Run("check list files", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/files", nil)
		response := httptest.NewRecorder()

		server := BuildServer(ServerConfig{
			filesStoragePath: "../test_files",
		})
		server.Handler.ServeHTTP(response, request)
		got := response.Body.String()
		log.Print(got)
		log.Print(response.Header())

	})
}
