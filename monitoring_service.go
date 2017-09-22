package main

import (
	"fmt"
	"github.com/Financial-Times/go-logger"
	"sort"
	"strings"
	"time"
)

const (
	defaultTimestampFormat    = time.RFC3339Nano
	contentType               = "Annotations"
	startEvent                = "PublishStart"
	completenessCriteriaEvent = "SaveNeo4j"
	endEvent                  = "PublishEnd"
	infoLevel                 = "info"
)

type MonitoringService interface {
	CloseCompletedTransactions()
	CloseSupersededTransactions(completedTids completedTransactionEvents, refInterval int)
	DetermineLookbackPeriod() int
}

type AnnotationsMonitoringService struct {
	eventReader               SplunkEventReader
	maxLookbackPeriod         int
	supersededCheckbackPeriod int
}

func (s AnnotationsMonitoringService) CloseCompletedTransactions() {

	lookbackTime := s.DetermineLookbackPeriod()

	// retrieve all the open transactions for a particular content type
	tids, err := s.eventReader.GetTransactions(strings.ToLower(contentType), fmt.Sprintf("%dm", lookbackTime))
	if err != nil {
		logger.Errorf(nil, err, "Monitoring transactions has failed.")
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
			} else if event.Event == completenessCriteriaEvent && event.Level == infoLevel {
				endTime = event.Time
			}

			// find mapper event: if message is not valid, log it as a PublishEnd event.
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
			logger.NewEntry(tid.TransactionID).WithUUID(tid.UUID).WithError(err).Error("Duration couldn't be determined, transaction won't be closed.")
		}

		completedTids = append(completedTids, completedTransactionEvent{tid.TransactionID, tid.UUID, tid.Duration, startTime, endTime})
		logger.Infof(map[string]interface{}{
			"@time":                endTime,
			"logTime":              time.Now().Format(defaultTimestampFormat),
			"event":                endEvent,
			"transaction_id":       tid.TransactionID,
			"uuid":                 tid.UUID,
			"startTime":            startTime,
			"endTime":              endTime,
			"transaction_duration": fmt.Sprint(duration.Seconds()),
			"monitoring_event":     "true",
			"isValid":              isValid,
			"content_type":         contentType,
		}, "Transaction has finished")
	}

	s.CloseSupersededTransactions(completedTids, lookbackTime)
}

func (s AnnotationsMonitoringService) DetermineLookbackPeriod() int {

	event, err := s.eventReader.GetLatestEvent(strings.ToLower(contentType), fmt.Sprintf("%dm", s.maxLookbackPeriod))
	if err != nil {
		return s.maxLookbackPeriod
	}

	t, err := time.Parse(defaultTimestampFormat, event.Time)
	if err != nil {
		return s.maxLookbackPeriod
	}

	//compute the time period since the last event was logged
	//consider that value - 5 min => to keep it overlapping
	period := time.Since(t)
	lookbackPeriod := period.Minutes() + 5
	if lookbackPeriod < 10 {
		lookbackPeriod = 10
	}

	return int(lookbackPeriod)
}

func (s AnnotationsMonitoringService) CloseSupersededTransactions(completedTids completedTransactionEvents, refInterval int) {

	//sort transactions
	sort.Sort(completedTids)

	// collect all the uuids that have successfully published in the recent transaction set
	var uuids []string
	for _, tid := range completedTids {
		uuids = append(uuids, tid.UUID)
	}

	// get all the uncompleted transactions for those UUIDs, that have started before our actual set
	unprocessedTids, err := s.eventReader.GetTransactionsForUUIDs(strings.ToLower(contentType), uuids, fmt.Sprintf("%dm", refInterval+s.supersededCheckbackPeriod))
	if err != nil {
		logger.Errorf(nil, err, "Checking for superseded transactions has failed.")
		return
	}

	// take all the completed transactions
	for _, ctid := range completedTids {

		// verify if within the unprocessed transactions there is any that have been superseded
		for i, utid := range unprocessedTids {
			if utid.UUID == ctid.UUID {

				//check that it is the same transaction: if so, remove it from the list
				if utid.TransactionID == ctid.TransactionID {
					unprocessedTids = append(unprocessedTids[:i], unprocessedTids[i+1:]...)
					continue
				}

				//check that it was a transaction that happened before the actual transaction
				if isEarlier, startTime := earlierTransaction(utid, ctid); isEarlier {

					duration, err := computeDuration(startTime, ctid.EndTime)
					if err != nil {
						logger.NewEntry(utid.TransactionID).WithUUID(utid.UUID).WithError(err).Error("Duration couldn't be determined, transaction won't be closed.")
					}

					logger.Infof(map[string]interface{}{
						"@time":                ctid.EndTime,
						"logTime":              time.Now().Format(defaultTimestampFormat),
						"event":                endEvent,
						"transaction_id":       utid.TransactionID,
						"uuid":                 utid.UUID,
						"startTime":            startTime,
						"endTime":              ctid.EndTime,
						"transaction_duration": fmt.Sprint(duration.Seconds()),
						"monitoring_event":     "true",
						// isValid field will be missing, because we can't tell for sure if that tid was failing
						// before it reached the mapper, of not. Also, we can't use the actual value for that tid, because the article
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

func earlierTransaction(utid transactionEvent, ctid completedTransactionEvent) (isEarlier bool, startTime string) {

	isAnnotationEvent := false
	isEarlier = false
	startTime = ""
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
