package main

import (
	"github.com/Financial-Times/go-logger"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
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
