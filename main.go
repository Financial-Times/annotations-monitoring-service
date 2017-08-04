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
		Value:  "http://localhost:8080/__splunk-event-reader/annotations/transactions",
		Desc:   "The address of the event reader application",
		EnvVar: "EVENT_READER_URL",
	})

	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
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

		go func() {
			serveAdminEndpoints(*appSystemCode, *appName, *port)
		}()

		monitorAnnotationsFlow(*eventReaderURL)
		waitForSignal()
	}
	err := app.Run(os.Args)
	if err != nil {
		logger.Errorf(nil, "App could not start, error=[%s]\n", err)
		return
	}
}

func serveAdminEndpoints(appSystemCode string, appName string, port string) {
	healthService := newHealthService(&healthConfig{appSystemCode: appSystemCode, appName: appName, port: port})

	serveMux := http.NewServeMux()

	hc := health.HealthCheck{SystemCode: appSystemCode, Name: appName, Description: appDescription, Checks: healthService.checks}
	serveMux.HandleFunc(healthPath, health.Handler(hc))
	serveMux.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.gtgCheck))
	serveMux.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	if err := http.ListenAndServe(":"+port, serveMux); err != nil {
		logger.FatalEvent("Unable to start: %v", err)
	}
}

func waitForSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
