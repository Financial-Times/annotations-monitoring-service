package main

import (
	"encoding/json"
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

func getLastEvent(eventReaderAddress string, interval string, lastEvent bool) (publishEvent, error) {
	req, err := http.NewRequest("GET", eventReaderAddress+"annotations/events", nil)

	q := req.URL.Query()
	q.Add(intervalPathVar, interval)
	q.Add(lastEventPathVar, strconv.FormatBool(lastEvent))
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url":         req.URL.String(),
			"status code": resp.StatusCode,
		}, "Failed to retrieve latest log event.", err)
		return publishEvent{}, err
	}
	defer cleanUp(resp)

	//TODO consider at least one retry?
	if resp.StatusCode != http.StatusOK {
		logger.Errorf(map[string]interface{}{
			"url":         req.URL.String(),
			"status code": resp.StatusCode,
		}, "Failed to retrieve latest log event")
		//retry? threat status codes accordingly 500 -> retry; 404->nil; 200->parse and continue;
		return publishEvent{}, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url":         req.URL.String(),
			"status code": resp.StatusCode,
		}, "Error parsing transaction body: %v", err)
		return publishEvent{}, err
	}

	var event publishEvent
	if err := json.Unmarshal(b, &event); err != nil {
		logger.Errorf(nil, "Error unmarshalling latest publish event message: %v", err)
		return publishEvent{}, err
	}

	return event, nil
}

func getTransactions(eventReaderAddress string, uuids []string, interval string) (transactions, error) {

	req, err := http.NewRequest("GET", eventReaderAddress+"annotations/transactions", nil)
	q := req.URL.Query()
	if uuids != nil && len(uuids) != 0 {
		for _, uuid := range uuids {
			q.Add(uuidPathVar, uuid)
		}
	}
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url":         req.URL.String(),
			"status code": resp.StatusCode,
		}, "Failed to retrieve transaction.", err)
		return nil, err
	}
	defer cleanUp(resp)

	//TODO consider at least one retry?
	if resp.StatusCode != http.StatusOK {
		logger.Errorf(map[string]interface{}{
			"url":         req.URL.String(),
			"status code": resp.StatusCode,
		}, "Failed to retrieve transations")
		//retry? threat status codes accordingly 500 -> retry; 404->nil; 200->parse and continue;
		return nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url":         req.URL.String(),
			"status code": resp.StatusCode,
		}, "Error parsing transaction body: %v", err)
		return nil, err
	}

	var tids transactions
	if err := json.Unmarshal(b, &tids); err != nil {
		logger.Errorf(nil, "Error unmarshalling transaction log messages: %v", err)
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
