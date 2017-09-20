package main

import (
	"fmt"
	"github.com/Financial-Times/go-logger"
	"sort"
	"time"
)

const (
	defaultTimestampFormat = time.RFC3339Nano
	isLastEventRequired    = true

	contentType = "Annotations"

	startEvent                = "PublishStart"
	completenessCriteriaEvent = "SaveNeo4j"
	endEvent                  = "PublishEnd"

	defaultCheckBackInterval       = "168h" // check back for 7 days at the most
	defaultMonitoringInterval      = "10m" // ? will this still be needed?
	defaultSupersededCheckInterval = "60m" // fix the last 60 minutes superseded
	defaultCheckFrequency          = 5 //

	infoLevel = "info"
)

type MonitoringService interface {
	StartMonitoring()
}

type AnnotationsMonitoringService struct {
	eventReaderURL string
}

func (s AnnotationsMonitoringService) StartMonitoring() {

	//start up from the last monitoring event in the store - if it is not present, consider a default interval
	s.catchUP()

	// check transactions every 5 minutes
	ticker := time.NewTicker(defaultCheckFrequency * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				//query for last 10 minutes
				// TODO add validation for interval
				// TODO change code, so that the normal monitoring event would look back from the LATEST PUBLISHEND event
				// - take the code from the catchUP method - ... refactor
				s.monitorTransactions(defaultMonitoringInterval)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func (s AnnotationsMonitoringService) getLookBackInterval() (int, error) {

	event, err := getLastEvent(s.eventReaderURL, defaultCheckBackInterval, isLastEventRequired)

	if err != nil {
		logger.Errorf(nil, err, "Error retrieving the latest monitoring publishEnd event from Splunk.")
		return 0, err
	}

	t, err := time.Parse(defaultTimestampFormat, event.Time)
	if err != nil {
		logger.Errorf(nil, err, "Error parsing the last event's timestamp.")
		return 0, err
	}

	//compute the duration since the last event was logged
	//consider that value - 5 min => to keep the overlapping period
	duration := time.Since(t)
	finalDuration := duration.Minutes() + 5
	if finalDuration < 10 {
		finalDuration = 10
	}

	m := int(finalDuration)

	return m, nil
}


func (s AnnotationsMonitoringService) catchUP() {

	m, err := s.getLookBackInterval()
	var interval string
	if err == nil {
		interval = fmt.Sprintf("%dm", m)
	} else {
		interval = defaultCheckBackInterval
	}

	fmt.Printf("Check back interval: %s", interval)

	s.monitorTransactions(interval)
}

func (s AnnotationsMonitoringService) monitorTransactions(interval string) {

	// retrieve all the entries for a particular content type
	tids, err := getTransactions(s.eventReaderURL, nil, interval)
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
			//TODO check whether this was intended or not
			logger.NewEntry(tid.TransactionID).WithUUID(tid.UUID).WithError(err).Error("Error parsing timestamp")
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

	sort.Sort(completedTids)
	fixSupersededTransactions(s.eventReaderURL, completedTids)
}

func fixSupersededTransactions(eventReaderAddress string, sortedCompletedTids completedTransactionEvents) {

	// check all transactions for that uuid that happened before

	// collect all the uuids that have successfully published in the recent transaction set
	var uuids []string
	for _, tid := range sortedCompletedTids {
		uuids = append(uuids, tid.UUID)
	}

	// get all the uncompleted transactions for those uuids, that have started before our actual set
	unprocessedTids, err := getTransactions(eventReaderAddress, uuids, defaultSupersededCheckInterval)
	if err != nil {
		logger.Errorf(nil, err, "Verification of possible superseded transactions has failed.")
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
						//TODO check whether this was intended or not
						logger.NewEntry(utid.TransactionID).WithUUID(utid.UUID).WithError(err).Error("Error parsing timestamp")
					}

					logger.Infof(map[string]interface{}{
						"@time":                ctid.EndTime,
						"logTime":              time.Now().Format(defaultTimestampFormat),
						"event":                endEvent,
						"transaction_id":       utid.TransactionID,
						"uuid":                 utid.UUID,
						"startTime":            t,
						"endTime":              ctid.EndTime,
						"transaction_duration": fmt.Sprint(duration.Seconds()),
						"monitoring_event":     "true",
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
