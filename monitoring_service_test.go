package main

import (
	"errors"
	"github.com/Financial-Times/go-logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"strings"
	"testing"
	"time"
)

func Test_CloseCompletedTransactions_Success(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	var readerMock = new(eventReaderMock)
	am := AnnotationsMonitoringService{
		eventReader:               readerMock,
		supersededCheckbackPeriod: 60,
	}

	tids := transactions{
		transactionEvent{
			TransactionID: "tid1",
			UUID:          "uuid1",
			Events: []publishEvent{
				{ContentType: "", Time: "2017-09-22T11:45:47.23038034Z", Event: startEvent},
				{ContentType: contentType, Time: "2017-09-22T11:45:49.23038034Z", IsValid: "true", Event: "Map"},
				{ContentType: contentType, Time: "2017-09-22T11:45:53.23038034Z", Event: completenessCriteriaEvent, Level: "info"},
			}}}

	readerMock.On("GetLatestEvent", strings.ToLower(contentType), mock.AnythingOfType("string")).
		Return(publishEvent{Time: time.Now().AddDate(0, 0, -1).Format(defaultTimestampFormat)}, nil).
		On("GetTransactions", strings.ToLower(contentType), "1445m").
		Return(tids, nil).
		On("GetTransactionsForUUIDs", strings.ToLower(contentType), []string{"uuid1"}, "1505m").
		Return(transactions{}, nil)

	am.CloseCompletedTransactions()

	// Verifications - check that the mock object was called with the previously specified parameters
	readerMock.AssertExpectations(t)

	// Verify that all the log message fields are as expected...
	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Equal(t, "Transaction has finished", hook.LastEntry().Message)
	assert.Equal(t, endEvent, hook.LastEntry().Data["event"])
	assert.Equal(t, "uuid1", hook.LastEntry().Data["uuid"])
	assert.Equal(t, contentType, hook.LastEntry().Data["content_type"])
	assert.Equal(t, "true", hook.LastEntry().Data["monitoring_event"])
	assert.Equal(t, "tid1", hook.LastEntry().Data["transaction_id"])

	assert.Equal(t, "2017-09-22T11:45:47.23038034Z", hook.LastEntry().Data["startTime"])
	assert.Equal(t, "2017-09-22T11:45:53.23038034Z", hook.LastEntry().Data["endTime"])

	assert.Equal(t, "true", hook.LastEntry().Data["isValid"])
	assert.Equal(t, "6", hook.LastEntry().Data["transaction_duration"])
	assert.True(t, hook.LastEntry().Data["@time"] != nil)
}

func Test_CloseCompletedTransactions_Timeout(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")
	var readerMock = new(eventReaderMock)
	am := AnnotationsMonitoringService{
		eventReader:               readerMock,
		supersededCheckbackPeriod: 60,
	}

	readerMock.On("GetLatestEvent", strings.ToLower(contentType), mock.AnythingOfType("string")).
		Return(publishEvent{Time: time.Now().AddDate(0, 0, -1).Format(defaultTimestampFormat)}, nil).
		On("GetTransactions", strings.ToLower(contentType), "1445m").
		Return(transactions{}, errors.New("timeout"))

	am.CloseCompletedTransactions()

	readerMock.AssertExpectations(t)
	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "Monitoring transactions has failed.", hook.LastEntry().Message)
}

func Test_CloseCompletedTransactions_WrongTimeFormat(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")
	var readerMock = new(eventReaderMock)
	am := AnnotationsMonitoringService{
		eventReader:               readerMock,
		supersededCheckbackPeriod: 60,
	}

	tids := transactions{
		transactionEvent{
			TransactionID: "tid1",
			UUID:          "uuid1",
			Events: []publishEvent{
				{ContentType: "", Time: "2017-09-22T11:45:47.23038034Z", Event: startEvent},
				{ContentType: contentType, Time: "2017-09-22T11:45:49.23038034Z", IsValid: "true", Event: "Map"},
				{ContentType: contentType, Time: "2017-09-22 11:45:00", Event: completenessCriteriaEvent, Level: "info"},
			}}}
	readerMock.On("GetLatestEvent", strings.ToLower(contentType), mock.AnythingOfType("string")).
		Return(publishEvent{Time: time.Now().AddDate(0, 0, -1).Format(defaultTimestampFormat)}, nil).
		On("GetTransactions", strings.ToLower(contentType), "1445m").
		Return(tids, nil)

	am.CloseCompletedTransactions()

	readerMock.AssertExpectations(t)
	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "Duration couldn't be determined, transaction won't be closed.", hook.LastEntry().Message)
}

func Test_CloseCompletedTransactions_NotAnnotationsMessage(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")
	var readerMock = new(eventReaderMock)
	am := AnnotationsMonitoringService{
		eventReader:               readerMock,
		supersededCheckbackPeriod: 60,
	}

	tids := transactions{
		transactionEvent{
			TransactionID: "tid1",
			UUID:          "uuid1",
			Events: []publishEvent{
				{ContentType: "", Time: "2017-09-22T11:45:47.23038034Z", Event: startEvent},
			}}}

	readerMock.On("GetLatestEvent", strings.ToLower(contentType), mock.AnythingOfType("string")).
		Return(publishEvent{Time: time.Now().AddDate(0, 0, -1).Format(defaultTimestampFormat)}, nil).
		On("GetTransactions", strings.ToLower(contentType), "1445m").
		Return(tids, nil)

	am.CloseCompletedTransactions()

	readerMock.AssertExpectations(t)
	assert.Equal(t, 0, len(hook.Entries))
}

func Test_CloseCompletedTransactions_Invalid_Message(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	var readerMock = new(eventReaderMock)
	am := AnnotationsMonitoringService{
		eventReader:               readerMock,
		supersededCheckbackPeriod: 60,
	}

	tids := transactions{
		transactionEvent{
			TransactionID: "tid1",
			UUID:          "uuid1",
			Events: []publishEvent{
				{ContentType: "", Time: "2017-09-22T11:45:47.23038034Z", Event: startEvent},
				{ContentType: contentType, Time: "2017-09-22T11:45:49.23038034Z", IsValid: "false", Event: "Map"},
			}}}

	readerMock.On("GetLatestEvent", strings.ToLower(contentType), mock.AnythingOfType("string")).
		Return(publishEvent{Time: time.Now().AddDate(0, 0, -1).Format(defaultTimestampFormat)}, nil).
		On("GetTransactions", strings.ToLower(contentType), "1445m").
		Return(tids, nil).
		On("GetTransactionsForUUIDs", strings.ToLower(contentType), []string{"uuid1"}, "1505m").
		Return(transactions{}, nil)

	am.CloseCompletedTransactions()

	readerMock.AssertExpectations(t)
	// Verify that all the log message fields are as expected...
	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Equal(t, "Transaction has finished", hook.LastEntry().Message)
	assert.Equal(t, endEvent, hook.LastEntry().Data["event"])
	assert.Equal(t, "uuid1", hook.LastEntry().Data["uuid"])
	assert.Equal(t, contentType, hook.LastEntry().Data["content_type"])
	assert.Equal(t, "true", hook.LastEntry().Data["monitoring_event"])
	assert.Equal(t, "tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "2017-09-22T11:45:47.23038034Z", hook.LastEntry().Data["startTime"])
	assert.Equal(t, "2017-09-22T11:45:49.23038034Z", hook.LastEntry().Data["endTime"])
	assert.Equal(t, "false", hook.LastEntry().Data["isValid"])
	assert.Equal(t, "2", hook.LastEntry().Data["transaction_duration"])
	assert.True(t, hook.LastEntry().Data["@time"] != nil)
}

func Test_CloseCompletedTransactions_ComplexScenario(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	var readerMock = new(eventReaderMock)
	am := AnnotationsMonitoringService{
		eventReader:               readerMock,
		supersededCheckbackPeriod: 60,
	}

	tids := transactions{
		// incomplete
		transactionEvent{
			TransactionID: "tid1",
			UUID:          "uuid1",
			StartTime:     "2017-09-22T11:45:00.00000000Z",
			Events: []publishEvent{
				{ContentType: "", Time: "2017-09-22T11:45:00.00000000Z", Event: startEvent},
				{ContentType: contentType, Time: "2017-09-22T11:45:02.00000000Z", IsValid: "true", Event: "Map"},
			}},
		// incomplete - arbitrary order
		transactionEvent{
			TransactionID: "tid3",
			UUID:          "uuid1",
			StartTime:     "2017-09-22T11:50:00.00000000Z",
			Events: []publishEvent{
				{ContentType: "", Time: "2017-09-22T11:50:00.00000000Z", Event: startEvent},
				{ContentType: contentType, Time: "2017-09-22T11:45:02.00000000Z", IsValid: "true", Event: "Map"},
			}},
		transactionEvent{
			TransactionID: "tid2",
			UUID:          "uuid1",
			StartTime:     "2017-09-22T11:47:00.00000000Z",
			Events: []publishEvent{
				{ContentType: "", Time: "2017-09-22T11:47:00.00000000Z", Event: startEvent},
				{ContentType: contentType, Time: "2017-09-22T11:45:02.00000000Z", IsValid: "true", Event: "Map"},
			}},
		// successful one
		transactionEvent{
			TransactionID: "tid4",
			UUID:          "uuid1",
			StartTime:     "2017-09-22T11:55:00.00000000Z",
			Events: []publishEvent{
				{ContentType: "", Time: "2017-09-22T11:55:00.00000000Z", Event: startEvent},
				{ContentType: contentType, Time: "2017-09-22T11:55:02.00000000Z", IsValid: "true", Event: "Map"},
				{ContentType: contentType, Time: "2017-09-22T11:55:04.00000000Z", Event: completenessCriteriaEvent, Level: "info"},
			}},
		// later successful one
		transactionEvent{
			TransactionID: "tid5",
			UUID:          "uuid1",
			StartTime:     "2017-09-22T11:56:00.00000000Z",
			Events: []publishEvent{
				{ContentType: "", Time: "2017-09-22T11:56:00.00000000Z", Event: startEvent},
				{ContentType: contentType, Time: "2017-09-22T11:56:02.00000000Z", IsValid: "true", Event: "Map"},
				{ContentType: contentType, Time: "2017-09-22T11:56:04.00000000Z", Event: completenessCriteriaEvent, Level: "info"},
			}},
	}

	readerMock.On("GetLatestEvent", strings.ToLower(contentType), mock.AnythingOfType("string")).
		Return(publishEvent{Time: time.Now().AddDate(0, 0, -1).Format(defaultTimestampFormat)}, nil).
		On("GetTransactions", strings.ToLower(contentType), "1445m").
		Return(tids, nil).
		On("GetTransactionsForUUIDs", strings.ToLower(contentType), mock.Anything, "1505m").
		Return(tids, nil)

	am.CloseCompletedTransactions()

	// Verifications - check that the mock object was called with the previously specified parameters
	readerMock.AssertExpectations(t)

	assert.Equal(t, 5, len(hook.AllEntries()))

	// Verify that all the log message fields are as expected...
	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Equal(t, "Transaction has been superseded by tid=tid4.", hook.LastEntry().Message)
	assert.Equal(t, endEvent, hook.LastEntry().Data["event"])
	assert.Equal(t, "uuid1", hook.LastEntry().Data["uuid"])
	assert.Equal(t, contentType, hook.LastEntry().Data["content_type"])
	assert.Equal(t, "true", hook.LastEntry().Data["monitoring_event"])
	assert.Equal(t, "tid3", hook.LastEntry().Data["transaction_id"])

	assert.Equal(t, "2017-09-22T11:50:00.00000000Z", hook.LastEntry().Data["startTime"])
	assert.Equal(t, "2017-09-22T11:55:04.00000000Z", hook.LastEntry().Data["endTime"])

	assert.Equal(t, nil, hook.LastEntry().Data["isValid"])
	assert.Equal(t, "304", hook.LastEntry().Data["transaction_duration"])
	assert.True(t, hook.LastEntry().Data["@time"] != nil)
}

func Test_CloseSupersededTransactions(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")
	var tests = []struct {
		scenarioName      string
		completedTids     []completedTransactionEvent
		superSeededPeriod int
		refInterval       int
		expContentType    string
		expUUIDs          []string
		expLookbackPeriod string
		resTransactions   transactions
		resError          error
		logLevel          string
		logMsg            string
	}{
		{
			"No completed TID, no event reader calls, no logs",
			[]completedTransactionEvent{}, 60, 60,
			"", nil, "",
			nil, nil,
			"none", "",
		},
		{
			"One uuid and transaction",
			[]completedTransactionEvent{
				{TransactionID: "tid1", UUID: "uuid1"},
			}, 60, 60,
			strings.ToLower(contentType), []string{"uuid1"}, "120m",
			transactions{}, nil,
			"none", "",
		},
		{
			"Multiple uuids, startTime ordering is required",
			[]completedTransactionEvent{
				{TransactionID: "tid2", UUID: "uuid2", StartTime: "2017-09-22T12:31:47.23038034Z"},
				{TransactionID: "tid1", UUID: "uuid1", StartTime: "2017-09-22T12:00:47.23038034Z"},
			}, 60, 60,
			strings.ToLower(contentType), []string{"uuid1", "uuid2"}, "120m",
			transactions{}, nil,
			"none", "",
		},
		{
			"For more transactions for the same uuid: send the uuid only one time for the reader-service request",
			[]completedTransactionEvent{
				{TransactionID: "tid2", UUID: "uuid1", StartTime: "2017-09-22T12:31:47.23038034Z"},
				{TransactionID: "tid1", UUID: "uuid1", StartTime: "2017-09-22T12:00:47.23038034Z"},
			}, 60, 60,
			strings.ToLower(contentType), []string{"uuid1"}, "120m",
			transactions{}, nil,
			"none", "",
		},
		{
			"Superseded call time out",
			[]completedTransactionEvent{
				{TransactionID: "tid2", UUID: "uuid2", StartTime: "2017-09-22T12:31:47.23038034Z"},
				{TransactionID: "tid1", UUID: "uuid1", StartTime: "2017-09-22T12:00:47.23038034Z"},
			}, 60, 60,
			strings.ToLower(contentType), []string{"uuid1", "uuid2"}, "120m",
			nil, errors.New("timed out"),
			"error", "Checking for superseded transactions has failed.",
		},
		{
			"Return unclosed tids for different uuids: ignore them",
			[]completedTransactionEvent{
				{TransactionID: "tid2", UUID: "uuid2", StartTime: "2017-09-22T12:31:47.23038034Z"},
				{TransactionID: "tid1", UUID: "uuid1", StartTime: "2017-09-22T12:00:47.23038034Z"},
			}, 60, 60,
			strings.ToLower(contentType), []string{"uuid1", "uuid2"}, "120m",
			transactions{
				transactionEvent{
					TransactionID: "tid3",
					UUID:          "uuid3",
					Events:        []publishEvent{{ContentType: contentType, Time: "2017-09-22T11:45:47.23038034Z", Event: startEvent}}},
			}, nil,
			"none", "",
		},
		{
			"Return the same transaction that has been logged as closed (the DB is behind): ignore it.",
			[]completedTransactionEvent{
				{TransactionID: "tid2", UUID: "uuid2", StartTime: "2017-09-22T12:31:47.23038034Z"},
				{TransactionID: "tid1", UUID: "uuid1", StartTime: "2017-09-22T12:00:47.23038034Z"},
			}, 60, 60,
			strings.ToLower(contentType), []string{"uuid1", "uuid2"}, "120m",
			transactions{
				transactionEvent{
					TransactionID: "tid1",
					UUID:          "uuid1",
					Events:        []publishEvent{{ContentType: contentType, Time: "2017-09-22T12:00:00.23038034Z", Event: startEvent}}},
			}, nil,
			"none", "",
		},
		{
			"Return later transactions for the same uuid: do nothing",
			[]completedTransactionEvent{
				{TransactionID: "tid2", UUID: "uuid2", StartTime: "2017-09-22T12:31:47.23038034Z"},
				{TransactionID: "tid1", UUID: "uuid1", StartTime: "2017-09-22T12:00:47.23038034Z"},
			}, 60, 60,
			strings.ToLower(contentType), []string{"uuid1", "uuid2"}, "120m",
			transactions{
				transactionEvent{
					TransactionID: "tid1_2",
					UUID:          "uuid1",
					Events:        []publishEvent{{ContentType: "notAnnotations", Time: "2017-09-22T11:45:47.23038034Z", Event: startEvent}}},
			}, nil,
			"none", "",
		},
		{
			"Return superseded values, but couldn't calculate duration: log error message, don't close transaction",
			[]completedTransactionEvent{
				{TransactionID: "tid2", UUID: "uuid2", StartTime: "2017-09-22T12:31:47.23038034Z"},
				{TransactionID: "tid1", UUID: "uuid1", StartTime: "2017-09-22T12:00:47.23038034Z"},
			}, 60, 60,
			strings.ToLower(contentType), []string{"uuid1", "uuid2"}, "120m",
			transactions{
				transactionEvent{
					TransactionID: "tid1_2",
					UUID:          "uuid1",
					Events:        []publishEvent{{ContentType: contentType, Time: "2017-09-22T11:45:47.23038034Z", Event: startEvent}}},
			}, nil,
			"error", "Duration couldn't be determined, transaction won't be closed.",
		},
		{
			"Return superseded values: log publishEnd event",
			[]completedTransactionEvent{
				{TransactionID: "tid2", UUID: "uuid2", StartTime: "2017-09-22T12:31:47.23038034Z", EndTime: "2017-09-22T12:31:49.23038034Z"},
				{TransactionID: "tid1", UUID: "uuid1", StartTime: "2017-09-22T12:00:47.23038034Z", EndTime: "2017-09-22T12:00:49.23038034Z"},
			}, 60, 60,
			strings.ToLower(contentType), []string{"uuid1", "uuid2"}, "120m",
			transactions{
				transactionEvent{
					TransactionID: "tid1_2",
					UUID:          "uuid1",
					Events:        []publishEvent{{ContentType: contentType, Time: "2017-09-22T11:45:47.23038034Z", Event: startEvent}}},
			}, nil,
			"info", "Transaction has been superseded by tid=tid1.",
		},
	}

	for _, test := range tests {

		hook.Reset()

		var readerMock = new(eventReaderMock)
		am := AnnotationsMonitoringService{
			eventReader:               readerMock,
			supersededCheckbackPeriod: test.superSeededPeriod,
		}

		// Set expectations for EventReader Mock object
		if len(test.completedTids) != 0 {
			readerMock.On("GetTransactionsForUUIDs", test.expContentType, test.expUUIDs, test.expLookbackPeriod).
				Return(test.resTransactions, test.resError)
		} else {
			readerMock.AssertNotCalled(t, "GetTransactionsForUUIDs")
		}

		// Execute superseded check operation
		am.CloseSupersededTransactions(test.completedTids, test.refInterval)

		// Verifications - check that the mock object was called with the previously specified parameters
		readerMock.AssertExpectations(t)

		// Verify that the log messages are the ones expected...
		if test.logLevel != "none" {
			assert.Equal(t, test.logLevel, hook.LastEntry().Level.String())
			assert.Equal(t, test.logMsg, hook.LastEntry().Message)
		} else {
			assert.Equal(t, 0, len(hook.Entries))
		}
	}
}

func Test_CloseSupersededTransactions_ComplexScenario(t *testing.T) {

	hook := logger.NewTestHook("annotations-monitoring-service")

	var readerMock = new(eventReaderMock)
	am := AnnotationsMonitoringService{
		eventReader:               readerMock,
		supersededCheckbackPeriod: 60,
	}

	completedTids := []completedTransactionEvent{
		{TransactionID: "tid2", UUID: "uuid2", StartTime: "2017-09-22T12:31:47.23038034Z", EndTime: "2017-09-22T12:31:49.23038034Z"},
		{TransactionID: "tid1", UUID: "uuid1", StartTime: "2017-09-22T12:00:47.23038034Z", EndTime: "2017-09-22T12:00:49.23038034Z"},
	}

	uuids := []string{"uuid1", "uuid2"}

	returnedTIDs := transactions{
		transactionEvent{
			TransactionID: "tid1_2",
			UUID:          "uuid1",
			Events:        []publishEvent{{ContentType: contentType, Time: "2017-09-22T11:45:47.23038034Z", Event: startEvent}}}}

	readerMock.On("GetTransactionsForUUIDs", strings.ToLower(contentType), uuids, "120m").
		Return(returnedTIDs, nil)

	am.CloseSupersededTransactions(completedTids, 60)

	// Verifications - check that the mock object was called with the previously specified parameters
	readerMock.AssertExpectations(t)

	// Verify that all the log message fields are as expected...
	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Equal(t, "Transaction has been superseded by tid=tid1.", hook.LastEntry().Message)
	assert.Equal(t, endEvent, hook.LastEntry().Data["event"])
	assert.Equal(t, "uuid1", hook.LastEntry().Data["uuid"])
	assert.Equal(t, contentType, hook.LastEntry().Data["content_type"])
	assert.Equal(t, "true", hook.LastEntry().Data["monitoring_event"])
	assert.Equal(t, "tid1_2", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "2017-09-22T11:45:47.23038034Z", hook.LastEntry().Data["startTime"])
	assert.Equal(t, "2017-09-22T12:00:49.23038034Z", hook.LastEntry().Data["endTime"])
	assert.Equal(t, "2017-09-22T12:00:49.23038034Z", hook.LastEntry().Data["@time"])
	assert.Equal(t, "902", hook.LastEntry().Data["transaction_duration"])
	assert.Equal(t, nil, hook.LastEntry().Data["isValid"]) //no isValid field should be added

	assert.True(t, hook.LastEntry().Data["logTime"] != nil)
}

func Test_DetermineLookbackPeriod(t *testing.T) {
	var tests = []struct {
		publishEvent            publishEvent
		transactions            transactions
		err                     error
		maxLookbackPeriod       int
		resultingLookbackPeriod int
	}{
		{publishEvent{}, transactions{}, nil, 60, 60},
		{publishEvent{}, transactions{}, errors.New("some error"), 60, 60},
		{publishEvent{Time: "2017-09-22T12:31:47.23038034Z"}, transactions{}, errors.New("some error"), 60, 60},
		{publishEvent{Time: time.Now().AddDate(0, 0, -1).Format(defaultTimestampFormat)}, transactions{}, nil, 60, 1445},
		{publishEvent{Time: time.Now().Add(-3 * time.Minute).Format(defaultTimestampFormat)}, transactions{}, nil, 60, 10},
	}

	for _, test := range tests {
		readerMock := new(eventReaderMock)
		am := AnnotationsMonitoringService{
			eventReader:       readerMock,
			maxLookbackPeriod: test.maxLookbackPeriod,
		}

		readerMock.On("GetLatestEvent", strings.ToLower(contentType), mock.AnythingOfType("string")).
			Return(test.publishEvent, test.err)

		lbp := am.DetermineLookbackPeriod()

		readerMock.AssertExpectations(t)
		assert.Equal(t, test.resultingLookbackPeriod, lbp)
	}
}

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

type eventReaderMock struct {
	mock.Mock
}

func (e *eventReaderMock) GetTransactions(contentType string, lookbackPeriod string) (transactions, error) {
	args := e.Called(contentType, lookbackPeriod)
	return args.Get(0).(transactions), args.Error(1)
}

func (e *eventReaderMock) GetTransactionsForUUIDs(contentType string, uuids []string, lookbackPeriod string) (transactions, error) {
	args := e.Called(contentType, uuids, lookbackPeriod)
	return args.Get(0).(transactions), args.Error(1)
}

func (e *eventReaderMock) GetLatestEvent(contentType string, lookbackPeriod string) (publishEvent, error) {
	args := e.Called(contentType, lookbackPeriod)
	return args.Get(0).(publishEvent), args.Error(1)
}
