package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Financial-Times/go-logger"
	"github.com/stretchr/testify/assert"
)

func Test_StartMonitoring(t *testing.T) {
	hook := logger.NewTestHook("annotations-monitoring-service")
	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer eventReaderServer.Close()
	startMonitoring(eventReaderServer.URL, 60, 30)
	assert.NotEmpty(t, hook.Entries)
}
