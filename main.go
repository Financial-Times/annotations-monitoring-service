package main

import (
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/jawher/mow.cli"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	appDescription = "Service responsible for monitoring annotations publishes."
	checkFrequency = 5 // check status of transactions every 5 minutes
)

func main() {
	app := cli.App("annotations-monitoring-service", appDescription)

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "annotations-monitoring-service",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})

	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "annotations-monitoring-service",
		Desc:   "Application name",
		EnvVar: "APP_NAME",
	})

	eventReaderURL := app.String(cli.StringOpt{
		Name:   "event-reader-url",
		Value:  "http://localhost:8083/__splunk-event-reader",
		Desc:   "The address of the event reader application",
		EnvVar: "EVENT_READER_URL",
	})

	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8084",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})

	maxLookbackPeriod := app.Int(cli.IntOpt{
		Name:   "maxLookbackPeriod",
		Value:  4320, // look back for 3 days at the most
		Desc:   "Defines (in minutes) how far should the monitoring service look back, if newer PublishEnd logs weren't found",
		EnvVar: "MAX_LOOKBACK_PERIOD",
	})

	supersededCheckbackPeriod := app.Int(cli.IntOpt{
		Name:   "defaultSupersededCheckPeriod",
		Value:  4320, // fix the last 3 days' superseded TIDs
		Desc:   "Defines (in minutes) how far should the monitoring service look back for fixing superseded articles.",
		EnvVar: "SUPERSEDED_CHECK_PERIOD",
	})

	logger.InitDefaultLogger(*appName)
	logger.Infof(nil, "[Startup] annotations-monitoring-service is starting ")

	app.Action = func() {
		logger.Infof(map[string]interface{}{
			"System code": *appSystemCode,
			"App Name":    *appName,
			"Port":        *port,
		}, "")

		go serveAdminEndpoints(*appSystemCode, *appName, *port, *eventReaderURL)
		startMonitoring(*eventReaderURL, *maxLookbackPeriod, *supersededCheckbackPeriod)

		waitForInterruptSignal()
	}
	err := app.Run(os.Args)
	if err != nil {
		logger.Errorf(nil, err, "App could not start")
		return
	}
}

func serveAdminEndpoints(appSystemCode, appName, port, eventReaderUrl string) {
	healthService := newHealthService(&healthConfig{appSystemCode, appName, port, eventReaderUrl})

	serveMux := http.NewServeMux()

	hc := health.HealthCheck{SystemCode: appSystemCode, Name: appName, Description: appDescription, Checks: healthService.checks}
	serveMux.HandleFunc(healthPath, health.Handler(hc))
	serveMux.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.gtgCheck))
	serveMux.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	server := http.Server{
		Addr:         ":" + port,
		Handler:      serveMux,
		ReadTimeout:  time.Duration(120 * time.Second),
		WriteTimeout: time.Duration(60 * time.Second),
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Fatalf(nil, err, "Unable to start service")
	}
}

func waitForInterruptSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

func startMonitoring(eventReaderURL string, maxLookbackPeriod, supersededCheckbackPeriod int) {
	as := AnnotationsMonitoringService{
		eventReader: SplunkEventReader{
			eventReaderAddress: eventReaderURL,
		},
		maxLookbackPeriod:         maxLookbackPeriod,
		supersededCheckbackPeriod: supersededCheckbackPeriod,
	}

	// close all the completed transactions that haven't yet been closed
	as.CloseCompletedTransactions()
	ticker := time.NewTicker(checkFrequency * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				as.CloseCompletedTransactions()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}
