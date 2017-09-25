package main

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

//type MonitoringService interface {
//	CloseCompletedTransactions()
//	CloseSupersededTransactions(completedTids completedTransactionEvents, refInterval int)
//	DetermineLookbackPeriod() int
//}

//type AnnotationsMonitoringService struct {
//	eventReader               SplunkEventReader
//	maxLookbackPeriod         int
//	supersededCheckbackPeriod int
//}

func Test_DetermineLookbackPeriod(t *testing.T) {
	var tests = []struct {
		eventReader             EventReader
		maxLookbackPeriod       int
		resultingLookbackPeriod int
	}{
		{mock_EventReader{publishEvent: publishEvent{}, transactions: transactions{}, err: nil}, 60, 60},
		{mock_EventReader{publishEvent: publishEvent{}, transactions: transactions{}, err: errors.New("some error")}, 60, 60},
		{mock_EventReader{publishEvent: publishEvent{Time: "2017-09-22T12:31:47.23038034Z"}, transactions: transactions{}, err: errors.New("some error")}, 60, 60},
		{mock_EventReader{publishEvent: publishEvent{Time: time.Now().AddDate(0, 0, -1).Format(defaultTimestampFormat)}, transactions: transactions{}, err: nil}, 60, 1445},
		{mock_EventReader{publishEvent: publishEvent{Time: time.Now().Add(-3 * time.Minute).Format(defaultTimestampFormat)}, transactions: transactions{}, err: nil}, 60, 10},
	}

	for _, test := range tests {
		am := AnnotationsMonitoringService{
			eventReader:       test.eventReader,
			maxLookbackPeriod: test.maxLookbackPeriod,
		}

		lbp := am.DetermineLookbackPeriod()
		assert.Equal(t, test.resultingLookbackPeriod, lbp)
	}
}

//func earlierTransaction(utid transactionEvent, ctid completedTransactionEvent) (isEarlier bool, startTime string) {
//
//	isAnnotationEvent := false
//	isEarlier = false
//	startTime = ""
//	for _, event := range utid.Events {
//		// mark as annotations event
//		if event.ContentType == contentType {
//			isAnnotationEvent = true
//		}
//		// find start event
//		if event.Event == startEvent && event.Time < ctid.StartTime {
//			isEarlier = true
//			startTime = event.Time
//		}
//	}
//
//	return isAnnotationEvent && isEarlier, startTime
//}

//type transactionEvent struct {
//	TransactionID string         `json:"transaction_id"`
//	UUID          string         `json:"uuid"`
//	ClosedTxn     string         `json:"closed_txn"`
//	Duration      string         `json:"duration"`
//	EventCount    string         `json:"eventcount"`
//	Events        []publishEvent `json:"events"`
//}
//type publishEvent struct {
//	ContentType     string `json:"content_type"`
//	Environment     string `json:"environment"`
//	Event           string `json:"event"`
//	IsValid         string `json:"isValid,omitempty"`
//	Level           string `json:"level"`
//	MonitoringEvent string `json:"monitoring_event"`
//	Msg             string `json:"msg"`
//	Platform        string `json:"platform"`
//	ServiceName     string `json:"service_name"`
//	Time            string `json:"@time"`
//	TransactionID   string `json:"transaction_id"`
//	UUID            string `json:"uuid"`
//}

func Test_earlierTransaction(t *testing.T) {
	var tests = []struct {
		unknown_tid   transactionEvent
		completed_tid completedTransactionEvent
		isEarlier     bool
		startTime     string
	}{
		{transactionEvent{}, completedTransactionEvent{}, false, ""},
		{transactionEvent{
			TransactionID: "tid1",
			UUID:          "uuid1",
			Events: []publishEvent{
				publishEvent{
					ContentType: "notAnnotation",
				},
			}},
			completedTransactionEvent{}, false, ""},
		{transactionEvent{
			TransactionID: "tid1",
			UUID:          "uuid1",
			Events: []publishEvent{
				publishEvent{
					ContentType: contentType,
					Event:       startEvent,
					Time:        "2017-09-22T12:31:47.23038034Z",
				},
			}},
			completedTransactionEvent{
				StartTime: "2017-09-22T12:32:47.23038034Z",
			}, true, "2017-09-22T12:31:47.23038034Z"},
	}

	for _, test := range tests {
		b, st := earlierTransaction(test.unknown_tid, test.completed_tid)
		assert.Equal(t, b, test.isEarlier)
		assert.Equal(t, st, test.startTime)
	}
}

func Test_computeDuration(t *testing.T) {
	var tests = []struct {
		startTime string
		endTime   string
		duration  int
		errMsg    string
	}{
		{"", "2017-09-22T12:31:47.23038034Z", 0, "cannot parse"},
		{"2017-09-22T12:31:47.23038034Z", "", 0, "cannot parse"},
		{"2017-09-22T12:31:47.23038034Z", "2017-09-22T12:36:47.23038034Z", 5, ""},
	}

	for _, test := range tests {
		d, e := computeDuration(test.startTime, test.endTime)
		assert.Equal(t, test.duration, int(d.Minutes()))
		if e != nil {
			assert.True(t, strings.Contains(e.Error(), test.errMsg))
		} else {
			assert.Equal(t, test.errMsg, "")
		}
	}
}

type mock_EventReader struct {
	publishEvent publishEvent
	transactions transactions
	err          error
}

func (e mock_EventReader) GetLatestEvent(contentType string, lookbackPeriod string) (publishEvent, error) {
	return e.publishEvent, e.err
}

func (e mock_EventReader) GetTransactions(contentType string, lookbackPeriod string) (transactions, error) {
	return e.transactions, e.err
}

func (e mock_EventReader) GetTransactionsForUUIDs(contentType string, uuids []string, lookbackPeriod string) (transactions, error) {
	return e.transactions, e.err
}
