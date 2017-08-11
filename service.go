package main

import (
	"encoding/json"
	"fmt"
	"github.com/Financial-Times/go-logger"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"time"
)

const (
	contentType            = "Annotations"
	uuidPathVar            = "uuid"
	DefaultTimestampFormat = time.RFC3339
	startEvent             = "PublishStart"
	lastEvent              = "SaveNeo4j"
	endEvent               = "PublishEnd"
)

func readEnabled(kAPI client.KeysAPI) bool {
	//TODO: check how many successful transactions are in 5 minutes in prod, how heavily would etcd be affected... would it handle so many requests?

	//By default, the cluster will be considered as active. Consider inactive only if ETCD value confirms (failovers).
	readEnabled := true

	resp, err := kAPI.Get(context.Background(), "/ft/healthcheck-categories/read/enabled", nil)
	if err != nil {
		logger.Errorf(nil, "Couldn't determine if the cluster is active. ETCD key can't be read. Error %v", err)
		return readEnabled
	}

	b, err := strconv.ParseBool(resp.Node.Value)
	if err != nil {
		logger.Errorf(nil, "Couldn't determine if the cluster is active. ETCD key can't be parsed. Error %v", err)
		return readEnabled
	}

	return b
}

func getTransactions(urlStr string, uuids []string) (transactions, error) {

	req, err := http.NewRequest("GET", urlStr, nil)

	if uuids != nil && len(uuids) != 0 {
		for _, uuid := range uuids {
			req.URL.Query().Add(uuidPathVar, uuid)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url":         req.Host + req.URL.Path,
			"status code": resp.StatusCode,
		}, "Failed to retrieve transaction.", err)
		return nil, err
	}
	defer cleanUp(resp)

	//TODO consider at least one retry?
	if resp.StatusCode != http.StatusOK {
		logger.Errorf(map[string]interface{}{
			"url":         req.Host + req.URL.Path,
			"status code": resp.StatusCode,
		}, "Failed to retrieve transations")
		//retry? threat status codes accordingly 500 -> retry; 404->nil; 200->parse and continue;
		return nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf(map[string]interface{}{
			"url":         req.Host + req.URL.Path,
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

func monitorTransactions(urlStr string, keyApi client.KeysAPI) {

	// retrieve all the entries for a particular content type
	tids, err := getTransactions(urlStr, nil)
	if err != nil {
		logger.Errorf(nil, "Monitoring transactions has failed.", err)
		return
	}

	var completedTids completedTransactionEvents

	for _, tid := range tids {

		var startTime, endTime, isValid string
		isAnnotationEvent := false

		for _, event := range tid.Events {
			// mark annotations event
			if event.ContentType == contentType {
				isAnnotationEvent = true
			}

			// find start or end event
			if event.Event == startEvent {
				startTime = event.Time
			} else if event.Event == lastEvent {
				endTime = event.Time
			}

			// find mapper event: if message is not valid, log it as successful PublishEnd event.
			// use isValid string, to distinguish between missing and invalid events
			if event.IsValid == "true" {
				isValid = "true"
			} else if event.IsValid == "false" {
				isValid = "false"
				endTime = event.Time
			}
		}

		//if it is not a completed and valid annotation transaction series: skip it
		if !isAnnotationEvent || startTime == "" || endTime == "" || isValid == "" {
			continue
		}

		duration, err := calculateDuration(startTime, endTime)
		if err != nil {
			logger.ErrorEventWithUUID(tid.TransactionID, tid.UUID, "Error parsing timestamp %v ", err)
		}

		readEnabled := readEnabled(keyApi)

		completedTids = append(completedTids, completedTransactionEvent{tid.TransactionID, tid.UUID, tid.Duration, startTime, endTime, readEnabled})
		logger.Infof(map[string]interface{}{
			"@time":                endTime,
			"event":                endEvent,
			"transaction_id":       tid.TransactionID,
			"uuid":                 tid.UUID,
			"startTime":            startTime,
			"endTime":              endTime,
			"transaction_duration": fmt.Sprint(duration.Seconds()),
			"read_enabled":         strconv.FormatBool(readEnabled),
			"monitoring_event":     "true",
			"isValid":              isValid,
			"content_type":         contentType,
		}, "Transaction has finished")
	}

	sort.Sort(completedTids)
	fixSupersededTransactions(urlStr, completedTids)
}

func calculateDuration(startTime, endTime string) (time.Duration, error) {
	et, err := time.Parse(DefaultTimestampFormat, endTime)
	if err != nil {
		return 0, err
	}
	st, err := time.Parse(DefaultTimestampFormat, startTime)
	if err != nil {
		return 0, err
	}
	return et.Sub(st), nil
}

func fixSupersededTransactions(urlStr string, sortedCompletedTids completedTransactionEvents) {

	// check all transactions for that uuid that happened before

	// collect all the uuids that have successfully published in the recent transaction set
	var uuids []string
	for _, tid := range sortedCompletedTids {
		uuids = append(uuids, tid.UUID)
	}

	// get all the uncompleted transactions for those uuids, that have started before our actual set
	unprocessedTids, err := getTransactions(urlStr, uuids)
	if err != nil {
		logger.Errorf(nil, "Verification of possible superseded transactions has failed.", err)
		return
	}

	// take all the completed transactions
	for _, ctid := range sortedCompletedTids {

		// check that unprocessed transactions
		for i, utid := range unprocessedTids {
			if utid.UUID == ctid.UUID {

				//check that it is the same transaction: if so, remove it from the store
				if utid.TransactionID == ctid.TransactionID {
					unprocessedTids = append(unprocessedTids[:i], unprocessedTids[i+1:]...)
					continue
				}

				//check that it was a transaction that happened before the actual transaction
				if b, t := earlierTransaction(utid, ctid); b {

					duration, err := calculateDuration(t, ctid.EndTime)
					if err != nil {
						logger.ErrorEventWithUUID(utid.TransactionID, utid.UUID, "Error parsing timestamp %v ", err)
					}

					logger.Infof(map[string]interface{}{
						"@time":                ctid.EndTime,
						"event":                endEvent,
						"transaction_id":       utid.TransactionID,
						"uuid":                 utid.UUID,
						"startTime":            t,
						"endTime":              ctid.EndTime,
						"transaction_duration": fmt.Sprint(duration.Seconds()),
						"monitoring_event":     "true",
						// the value from the completed transaction will be used since it is impossible to detect
						// whether the cluster was active or not, when the transaction has started
						"read_enabled": strconv.FormatBool(ctid.ReadEnabled),
						// isValid field will be missing, because we can't tell for sure if that tid was failing
						// before it reached the mapper, of not. Also, we can't use the actual value for that tid, cause the article
						// might have suffered validation changes by then.
						"content_type": contentType,
					}, "Transaction has finished")

					//remove from unprocessedTransactionList
					unprocessedTids = append(unprocessedTids[:i], unprocessedTids[i+1:]...)
				}
			}
		}
	}
}

func earlierTransaction(utid transactionEvent, ctid completedTransactionEvent) (bool, string) {

	isAnnotationEvent := false
	isEarlier := false
	startTime := ""
	for _, event := range utid.Events {
		// mark as annotations event
		if event.ContentType == contentType {
			isAnnotationEvent = true
		}
		// find start event
		if event.Event == startEvent && event.Time < ctid.StartTime {
			isEarlier = true
			startTime = event.Time
		}
	}

	return isAnnotationEvent && isEarlier, startTime
}

func catchUP() {
	//TO implement
}

func monitorAnnotationsFlow(url string, keyAPI client.KeysAPI) {
	// every 5 minutes
	catchUP()

	ticker := time.NewTicker(5 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				//query for last 10 minutes
				monitorTransactions(url, keyAPI)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}
