package main

import (
	"encoding/json"
	"errors"
	"github.com/Financial-Times/go-logger"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
)

const (
	uuidPathVar      = "uuid"
	intervalPathVar  = "interval"
	lastEventPathVar = "lastEvent"
)

type EventReader interface {
	GetTransactions(contentType string, lookbackPeriod string) (transactions, error)
	GetTransactionsForUUIDs(contentType string, uuids []string, lookbackPeriod string) (transactions, error)
	GetLatestEvent(contentType string, lookbackPeriod string) (publishEvent, error)
}

type SplunkEventReader struct {
	eventReaderAddress string
}

func (ser SplunkEventReader) GetLatestEvent(contentType string, lookbackPeriod string) (publishEvent, error) {
	req, err := http.NewRequest("GET", ser.eventReaderAddress+"/"+contentType+"/events", nil)

	q := req.URL.Query()
	q.Add(intervalPathVar, lookbackPeriod)
	q.Add(lastEventPathVar, strconv.FormatBool(true))
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url": req.URL.String(),
		}, err, "Failed to retrieve latest log event")
		return publishEvent{}, err
	}
	defer cleanUp(resp)

	if resp.StatusCode != http.StatusOK {
		logger.Errorf(map[string]interface{}{
			"url":         req.URL.String(),
			"status code": resp.StatusCode,
		}, nil, "Failed to retrieve latest log event")
		return publishEvent{}, errors.New("Failed to retrieve latest log event")
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url": req.URL.String(),
		}, err, "Error parsing transaction body")
		return publishEvent{}, err
	}

	var event publishEvent
	if err := json.Unmarshal(b, &event); err != nil {
		logger.Errorf(map[string]interface{}{
			"url": req.URL.String(),
		}, err, "Error unmarshalling latest publish event message")
		return publishEvent{}, err
	}

	return event, nil
}

func (ser SplunkEventReader) GetTransactions(contentType string, lookbackPeriod string) (transactions, error) {
	return ser.GetTransactionsForUUIDs(contentType, nil, lookbackPeriod)
}

func (ser SplunkEventReader) GetTransactionsForUUIDs(contentType string, uuids []string, interval string) (transactions, error) {

	req, err := http.NewRequest("GET", ser.eventReaderAddress+"/"+contentType+"/transactions", nil)
	q := req.URL.Query()
	if uuids != nil && len(uuids) != 0 {
		for _, uuid := range uuids {
			q.Add(uuidPathVar, uuid)
		}
	}
	q.Add(intervalPathVar, interval)
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url": req.URL.String(),
		}, err, "Failed to retrieve transactions")
		return nil, err
	}
	defer cleanUp(resp)

	if resp.StatusCode != http.StatusOK {
		logger.Errorf(map[string]interface{}{
			"url":         req.URL.String(),
			"status code": resp.StatusCode,
		}, nil, "Failed to retrieve transactions")
		return nil, errors.New("Failed to retrieve transactions")
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url": req.URL.String(),
		}, err, "Error parsing transaction body")
		return nil, err
	}

	var tids transactions
	if err := json.Unmarshal(b, &tids); err != nil {
		logger.Errorf(map[string]interface{}{
			"url": req.URL.String(),
		}, err, "Error unmarshalling transaction log messages")
		return nil, err
	}

	return tids, nil
}

func cleanUp(resp *http.Response) {
	_, err := io.Copy(ioutil.Discard, resp.Body)
	if err != nil {
		logger.Warnf(nil, "[%v]", err)
	}

	err = resp.Body.Close()
	if err != nil {
		logger.Warnf(nil, "[%v]", err)
	}
}
