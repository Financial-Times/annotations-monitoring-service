package main

import (
	"fmt"
	"github.com/Financial-Times/go-logger"
	"github.com/coreos/etcd/client"
	"sort"
	"strconv"
	"time"
)

const (
	contentType               = "Annotations"
	defaultTimestampFormat    = time.RFC3339
	startEvent                = "PublishStart"
	lastEvent                 = "SaveNeo4j"
	endEvent                  = "PublishEnd"
	defaultCheckBackInterval  = "6h"
	defaultMonitoringInterval = "10m"
	isLastEventRequired       = true
)

func monitorAnnotationsFlow(eventReaderAddress string, readEnabledKey string, keyAPI client.KeysAPI) {

	catchUP(eventReaderAddress, readEnabledKey, keyAPI)

	// check transations every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				//query for last 10 minutes
				// TODO add validation for interval
				// TODO change code, so that the normal monitoring event would look back from the LATEST PUBLISHEND event
				// - take the code from the catchUP method - ... refactor
				monitorTransactions(eventReaderAddress, readEnabledKey, keyAPI, defaultMonitoringInterval)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func catchUP(eventReaderAddress string, readEnabledKey string, keyAPI client.KeysAPI) {

	event, err := getLastEvent(eventReaderAddress, defaultCheckBackInterval, isLastEventRequired)
	if err != nil {
		logger.Errorf(nil, "Error retrieving the latest monitoring publishEnd event from Splunk.", err)
		return
	}

	t, err := time.Parse(defaultTimestampFormat, event.Time)
	if err != nil {
		logger.Errorf(nil, "Error parsing the last event's timestamp %v ", err)
	}

	//compute the duration since the last event was logged
	//consider that value - 5 min => to keep the overlapping period
	duration := time.Since(t)
	finalDuration := duration.Minutes() + 5
	if finalDuration < 10 {
		finalDuration = 10
	}

	m := int(finalDuration)
	interval := fmt.Sprintf("%dm", m)

	monitorTransactions(eventReaderAddress, readEnabledKey, keyAPI, interval)
}

func monitorTransactions(eventReaderAddress string, readEnabledKey string, keyApi client.KeysAPI, interval string) {

	// retrieve all the entries for a particular content type
	tids, err := getTransactions(eventReaderAddress, nil, interval)
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

		duration, err := computeDuration(startTime, endTime)
		if err != nil {
			logger.ErrorEventWithUUID(tid.TransactionID, tid.UUID, "Error parsing timestamp %v ", err)
		}

		//TODO: check how many successful transactions are in 5/10 minutes in prod, how heavily would etcd be requested...
		//would it handle so many requests?
		readEnabled := readEnabled(keyApi,readEnabledKey)

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
	fixSupersededTransactions(eventReaderAddress, completedTids)
}

func fixSupersededTransactions(eventReaderAddress string, sortedCompletedTids completedTransactionEvents) {

	// check all transactions for that uuid that happened before

	// collect all the uuids that have successfully published in the recent transaction set
	var uuids []string
	for _, tid := range sortedCompletedTids {
		uuids = append(uuids, tid.UUID)
	}

	// get all the uncompleted transactions for those uuids, that have started before our actual set
	unprocessedTids, err := getTransactions(eventReaderAddress, uuids, defaultMonitoringInterval)
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

					duration, err := computeDuration(t, ctid.EndTime)
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

func computeDuration(startTime, endTime string) (time.Duration, error) {
	et, err := time.Parse(defaultTimestampFormat, endTime)
	if err != nil {
		return 0, err
	}
	st, err := time.Parse(defaultTimestampFormat, startTime)
	if err != nil {
		return 0, err
	}
	return et.Sub(st), nil
}
