package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventReaderReachabilityChecker_ServiceUnavailable(t *testing.T) {

	splunkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer splunkServer.Close()

	healthService := newHealthService(&healthConfig{eventReaderUrl: splunkServer.URL})
	message, err := healthService.eventReaderReachabilityChecker()

	assert.Equal(t, fmt.Sprintf("Connecting to %s/__gtg was not successful. Status: %d", splunkServer.URL, http.StatusServiceUnavailable), message)
	assert.Equal(t, fmt.Errorf("Status: %d", http.StatusServiceUnavailable), err)
}

func TestEventReaderReachabilityChecker_RequestError(t *testing.T) {

	splunkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	splunkServer.Close()

	healthService := newHealthService(&healthConfig{eventReaderUrl: splunkServer.URL})
	message, err := healthService.eventReaderReachabilityChecker()

	assert.Equal(t, fmt.Sprintf("Error executing requests for url=%s/__gtg", splunkServer.URL), message)
	assert.NotNil(t, err)
}

func TestEventReaderReachabilityChecker_Healthy(t *testing.T) {

	splunkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))
	defer splunkServer.Close()

	healthService := newHealthService(&healthConfig{eventReaderUrl: splunkServer.URL})
	message, err := healthService.eventReaderReachabilityChecker()

	assert.Equal(t, "Splunk event reader is healthy", message)
	assert.Nil(t, err)
}

func TestEventReader_GTG_Succeeds(t *testing.T) {

	splunkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer splunkServer.Close()

	healthService := newHealthService(&healthConfig{eventReaderUrl: splunkServer.URL})
	status := healthService.gtgCheck()

	assert.Equal(t, "", status.Message)
	assert.Equal(t, true, status.GoodToGo)
}

func TestEventReader_GTG_Fails(t *testing.T) {

	splunkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	splunkServer.Close()

	healthService := newHealthService(&healthConfig{eventReaderUrl: splunkServer.URL})
	status := healthService.gtgCheck()

	assert.NotNil(t, status.Message)
	assert.Equal(t, false, status.GoodToGo)
}
