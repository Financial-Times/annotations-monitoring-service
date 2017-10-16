package main

import (
	"fmt"
	"github.com/Financial-Times/go-logger"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetLatestEvent_ServerErrors(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/%s/events", strings.ToLower(contentType)), r.URL.Path)
		assert.Equal(t, fmt.Sprintf("%s=%s&%s=%s", intervalPathVar, "60m", lastEventPathVar, "true"), r.URL.RawQuery)

		w.WriteHeader(http.StatusInternalServerError)

	}))
	defer eventReaderServer.Close()

	eventReader := SplunkEventReader{
		eventReaderAddress: eventReaderServer.URL,
	}

	res, err := eventReader.GetLatestEvent(strings.ToLower(contentType), "60m")

	assert.Equal(t, publishEvent{}, res)
	assert.NotNil(t, err)

	//log message format
	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "Failed to retrieve latest log event", hook.LastEntry().Message)
	assert.Equal(t, fmt.Sprintf("%s/%s/events?interval=60m\u0026lastEvent=true", eventReaderServer.URL, strings.ToLower(contentType)), hook.LastEntry().Data["url"])
}

func TestGetLatestEvent_5xx(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	eventReaderServer.Close()

	eventReader := SplunkEventReader{
		eventReaderAddress: eventReaderServer.URL,
	}

	res, err := eventReader.GetLatestEvent(strings.ToLower(contentType), "60m")

	assert.Equal(t, publishEvent{}, res)
	assert.NotNil(t, err)

	//log message format
	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "Failed to retrieve latest log event", hook.LastEntry().Message)
	assert.Equal(t, fmt.Sprintf("%s/%s/events?interval=60m\u0026lastEvent=true", eventReaderServer.URL, strings.ToLower(contentType)), hook.LastEntry().Data["url"])
}

func TestGetLatestEvent_UnmarshallingError(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/%s/events", strings.ToLower(contentType)), r.URL.Path)
		assert.Equal(t, fmt.Sprintf("%s=%s&%s=%s", intervalPathVar, "60m", lastEventPathVar, "true"), r.URL.RawQuery)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Wrong body format"))

	}))
	defer eventReaderServer.Close()

	eventReader := SplunkEventReader{
		eventReaderAddress: eventReaderServer.URL,
	}

	res, err := eventReader.GetLatestEvent(strings.ToLower(contentType), "60m")

	assert.Equal(t, publishEvent{}, res)
	assert.NotNil(t, err)

	//log message format
	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "Error unmarshalling latest publish event message", hook.LastEntry().Message)
	assert.Equal(t, fmt.Sprintf("%s/%s/events?interval=60m\u0026lastEvent=true", eventReaderServer.URL, strings.ToLower(contentType)), hook.LastEntry().Data["url"])
}

func TestGetLatestEvent_Success(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/%s/events", strings.ToLower(contentType)), r.URL.Path)
		assert.Equal(t, fmt.Sprintf("%s=%s&%s=%s", intervalPathVar, "60m", lastEventPathVar, "true"), r.URL.RawQuery)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))

	}))
	defer eventReaderServer.Close()

	eventReader := SplunkEventReader{
		eventReaderAddress: eventReaderServer.URL,
	}

	res, err := eventReader.GetLatestEvent(strings.ToLower(contentType), "60m")

	assert.Equal(t, publishEvent{}, res)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(hook.Entries))
}

func TestGetTransactionsForUUIDs_UnmarshallingError(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/%s/transactions", strings.ToLower(contentType)), r.URL.Path)
		assert.Equal(t, fmt.Sprintf("%s=%s", intervalPathVar, "60m"), r.URL.RawQuery)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Wrong body format"))
	}))
	defer eventReaderServer.Close()

	eventReader := SplunkEventReader{
		eventReaderAddress: eventReaderServer.URL,
	}

	res, err := eventReader.GetTransactionsForUUIDs(strings.ToLower(contentType), []string{}, "60m")

	assert.Nil(t, res)
	assert.NotNil(t, err)

	//log message format
	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "Error unmarshalling transaction log messages", hook.LastEntry().Message)
	assert.Equal(t, fmt.Sprintf("%s/%s/transactions?interval=60m", eventReaderServer.URL, strings.ToLower(contentType)), hook.LastEntry().Data["url"])
}

func TestGetTransactionsForUUIDs_ServerErrors(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	eventReaderServer.Close()

	eventReader := SplunkEventReader{
		eventReaderAddress: eventReaderServer.URL,
	}

	res, err := eventReader.GetTransactionsForUUIDs(strings.ToLower(contentType), []string{}, "60m")

	assert.Nil(t, res)
	assert.NotNil(t, err)

	//log message format
	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "Failed to retrieve transactions", hook.LastEntry().Message)
	assert.Equal(t, fmt.Sprintf("%s/%s/transactions?interval=60m", eventReaderServer.URL, strings.ToLower(contentType)), hook.LastEntry().Data["url"])
}

func TestGetTransactionsForUUIDs_5xx(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/%s/transactions", strings.ToLower(contentType)), r.URL.Path)
		assert.Equal(t, fmt.Sprintf("%s=%s", intervalPathVar, "60m"), r.URL.RawQuery)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer eventReaderServer.Close()

	eventReader := SplunkEventReader{
		eventReaderAddress: eventReaderServer.URL,
	}

	res, err := eventReader.GetTransactionsForUUIDs(strings.ToLower(contentType), []string{}, "60m")

	assert.Nil(t, res)
	assert.NotNil(t, err)

	//log message format
	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "Failed to retrieve transactions", hook.LastEntry().Message)
	assert.Equal(t, fmt.Sprintf("%s/%s/transactions?interval=60m", eventReaderServer.URL, strings.ToLower(contentType)), hook.LastEntry().Data["url"])
}

func TestGetTransactionsForUUIDs_Success(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")
	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/%s/transactions", strings.ToLower(contentType)), r.URL.Path)
		assert.Equal(t, fmt.Sprintf("%s=%s&%s=%s&%s=%s", intervalPathVar, "60m", uuidPathVar, "uuid1", uuidPathVar, "uuid2"), r.URL.RawQuery)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer eventReaderServer.Close()

	eventReader := SplunkEventReader{
		eventReaderAddress: eventReaderServer.URL,
	}

	res, err := eventReader.GetTransactionsForUUIDs(strings.ToLower(contentType), []string{"uuid1", "uuid2"}, "60m")

	assert.Equal(t, transactions{}, res)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(hook.Entries))
}

func TestGetTransactions_Success(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")
	eventReaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("/%s/transactions", strings.ToLower(contentType)), r.URL.Path)
		assert.Equal(t, fmt.Sprintf("%s=%s", intervalPathVar, "60m"), r.URL.RawQuery)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer eventReaderServer.Close()

	eventReader := SplunkEventReader{
		eventReaderAddress: eventReaderServer.URL,
	}

	res, err := eventReader.GetTransactions(strings.ToLower(contentType), "60m")

	assert.Equal(t, transactions{}, res)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(hook.Entries))
}
