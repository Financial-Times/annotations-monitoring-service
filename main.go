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

const appDescription = "Service responsible for monitoring annotations publishes."

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

	logger.InitDefaultLogger(*appName)
	logger.Infof(nil, "[Startup] annotations-monitoring-service is starting ")

	app.Action = func() {
		logger.Infof(map[string]interface{}{
			"System code": *appSystemCode,
			"App Name":    *appName,
			"Port":        *port,
		}, "")

		as := AnnotationsMonitoringService{
			eventReaderURL: *eventReaderURL,
		}

		go func() {
			serveAdminEndpoints(*appSystemCode, *appName, *port, *eventReaderURL)
		}()

		as.StartMonitoring()

		waitForSignal()
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
		ReadTimeout:  time.Duration(60 * time.Second),
		WriteTimeout: time.Duration(60 * time.Second),
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Fatalf(nil, err, "Unable to start service")
	}
}

func waitForSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
