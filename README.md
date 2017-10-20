# annotations-monitoring-service

[![Circle CI](https://circleci.com/gh/Financial-Times/annotations-monitoring-service/tree/master.png?style=shield)](https://circleci.com/gh/Financial-Times/annotations-monitoring-service/tree/master)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/annotations-monitoring-service)](https://goreportcard.com/report/github.com/Financial-Times/annotations-monitoring-service) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/annotations-monitoring-service/badge.svg)](https://coveralls.io/github/Financial-Times/annotations-monitoring-service)

## Introduction

Service responsible for monitoring annotations publishes.

## Installation

Download the source code, dependencies and test dependencies:

        go get -u github.com/kardianos/govendor
        go get -u github.com/Financial-Times/annotations-monitoring-service
        cd $GOPATH/src/github.com/Financial-Times/annotations-monitoring-service
        govendor sync
        go build .

## Running locally

1. Run the tests and install the binary:

        govendor sync
        govendor test -v -race
        go install

2. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/annotations-monitoring-service [--help]

Options:

        --app-system-code="annotations-monitoring-service"                      System Code of the application ($APP_SYSTEM_CODE)
        --app-name="annotations-monitoring-service"                             Application name ($APP_NAME)
        --port="8080"                                                           Port to listen on ($APP_PORT)
        --event-reader-url="http://localhost:8083/__splunk-event-reader"        The address of the event reader application ($EVENT_READER_URL)
        --maxLookbackPeriodMin="4320"                                           Lookback period (in minutes), with a 3 days default ($MAX_LOOKBACK_PERIOD_MIN)
        --defaultSupersededCheckbackPeriodMin="4320"                            Lookback period (in minutes) for superseding checks, with a 3 days default ($SUPERSEDED_CHECK_PERIOD_MIN)
        
## Build and deployment

* Built by Docker Hub on merge to master: [coco/annotations-monitoring-service](https://hub.docker.com/r/coco/annotations-monitoring-service/)
* CI provided by CircleCI: [annotations-monitoring-service](https://circleci.com/gh/Financial-Times/annotations-monitoring-service)

## Service details

The annotations monitoring service is responsible for closing (logging a PublishEnd event for) completed transactions when publishing an annotation.
The basic algorithm:

        every 5 minutes repeat {
                1) call splunk-event-reader to determine the lookbackPeriod (last successful PublishEnd event)
                2) call splunk-event-reader to receive all the open annotations transactions for the lookbackPeriod (transactions with no PublishEnd event)
                3) close completed transactions (valid annotation messages with successful Neo4j write event)
                4) call splunk-event-reader to receive earlier unclosed transactions - check for lookbackPeriod + supersededLookbackPeriod
                5) close events that have been superseded by recent publishes
        }

The [annotations monitoring documentation](https://docs.google.com/document/d/1al-fcaoAg2RgmW2zzpkN6E91jge80wFpbV_ywGE86NA/edit#heading=h.6p6dv4ugzke2) explains the flow and the service functionality in more detail.

## Healthchecks
Admin endpoints are:

`/__gtg`

`/__health`

`/__build-info`

The health of the system indicates whether the underlying splunk-event-reader service is available.

### Logging

* The application uses the FT [go-logger](https://github.com/Financial-Times/go-logger) library (based on the [logrus](https://github.com/Sirupsen/logrus) implementation).
* The application uses specific monitoring log format, when logging a PublishEnd event for a complete transaction.
* NOTE: `/__build-info` and `/__gtg` endpoints are not logged as they are called every second from varnish/vulcand and this information is not needed in logs/splunk.