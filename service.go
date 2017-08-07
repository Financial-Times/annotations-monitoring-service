package main

import (
	"encoding/json"
	"github.com/Financial-Times/go-logger"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
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

func getTransactions(urlStr string, uuids []string) (transactions, error) {

	logger.Infof(nil, "url: %s", urlStr)
	req, err := http.NewRequest("GET", urlStr, nil)

	if uuids != nil && len(uuids) != 0 {
		for _, uuid := range uuids {
			req.URL.Query().Add(uuidPathVar, uuid)
		}
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		logger.Errorf(nil, "error: %v", err)
		return nil, err
	}
	defer cleanUp(resp)

	if resp.StatusCode != http.StatusOK {
		logger.Errorf(nil, "statuscode: %v", resp.StatusCode)
		//retry? threat status codes accordingly 500 -> retry; 404->nil; 200->parse and continue;
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf(nil, "error: %v", err)
	}

	var tids transactions
	if err := json.Unmarshal(b, &tids); err != nil {
		logger.Errorf(nil, "error: %v", err)
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

func monitorTransactions(urlStr string) {

	// retrieve all the entries for a particular content type
	tids, err := getTransactions(urlStr, nil)
	if err != nil {
		logger.Errorf(nil, "error: %v", err)
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
			if event.Event == startEvent { //PublishStart
				startTime = event.Time
			} else if event.Event == lastEvent {
				endTime = event.Time
			}

			// find mapper event: if not valid, log it as successful
			if event.IsValid == "true" {
				isValid = "true"
			} else if event.IsValid == "false" {
				isValid = "false"
				endTime = event.Time
				break
			}
		}

		if !isAnnotationEvent {
			continue
		}

		//message not valid, but successfully terminated
		if isValid == "false" { //what if no startTime, cause that's in a separate block? will the uuid part get it later on?
			if startTime != "" {
				completedTids = append(completedTids, completedTransactionEvent{tid.TransactionID, tid.UUID, tid.Duration, startTime, endTime})
				logger.Infof(map[string]interface{}{
					"event":            endEvent,
					"uuid":             tid.UUID,
					"startTime":        startTime,
					"endTime":          endTime,
					"transaction_id":   tid.TransactionID,
					"read_enabled":     "true",
					"monitoring_event": "true",
					"isValid":          "false",
					"content_type":     contentType,
				}, "Transaction has finished")
				continue
			}
		}

		if isValid == "true" && endTime != "" && startTime != "" {
			// TODO handle errors
			et, _ := time.Parse(DefaultTimestampFormat, endTime)
			st, _ := time.Parse(DefaultTimestampFormat, startTime)
			duration := et.Sub(st)

			completedTids = append(completedTids, completedTransactionEvent{tid.TransactionID, tid.UUID, tid.Duration, startTime, endTime})
			logger.Infof(map[string]interface{}{
				"event":            endEvent,
				"transaction_id":   tid.TransactionID,
				"uuid":             tid.UUID,
				"startTime":        startTime,
				"endTime":          endTime,
				"duration":         duration.Seconds(),
				"content_type":     contentType,
				"monitoring_event": "true",
				"read_enabled":     "true"}, "Transaction has finished")
		}
	}

	sort.Sort(completedTids)
	fixSuperseededTransactions(urlStr, completedTids)
}

func fixSuperseededTransactions(urlStr string, sortedCompletedTids completedTransactionEvents) {

	// check all transactions for that uuid have happened before
	var uuids []string
	for _, tid := range sortedCompletedTids {
		uuids = append(uuids, tid.UUID)
	}

	// transactions that happened before...
	unprocessedTids, err := getTransactions(urlStr, uuids)
	if err != nil {
		//log error
		logger.Errorf(nil, "error: %v", err)
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
					// TODO handle errors
					et, _ := time.Parse(DefaultTimestampFormat, ctid.EndTime)
					st, _ := time.Parse(DefaultTimestampFormat, t)
					duration := et.Sub(st)
					logger.Infof(map[string]interface{}{
						"event":            endEvent,
						"transaction_id":   utid.TransactionID,
						"uuid":             utid.UUID,
						"startTime":        t,
						"endTime":          ctid.EndTime,
						"content_type":     contentType,
						"duration":         duration.Seconds(),
						"monitoring_event": "true",
						"read_enabled":     "true"}, "Transaction has finished")

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

func monitorAnnotationsFlow(url string) {
	// every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				//query for last 10 minutes
				monitorTransactions(url)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}
