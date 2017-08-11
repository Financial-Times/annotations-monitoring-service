package main

import (
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/coreos/etcd/client"
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
		Value:  "http://localhost:8080/",
		Desc:   "The address of the event reader application",
		EnvVar: "EVENT_READER_URL",
	})

	etcdURL := app.String(cli.StringOpt{
		Name:   "etcd-url",
		Value:  "http://127.0.0.1:4001",
		Desc:   "The address of the etcd server",
		EnvVar: "ETCD_URL",
	})

	etcdKey := app.String(cli.StringOpt{
		Name:   "read-enabled-key",
		Value:  "/ft/healthcheck-categories/read/enabled",
		Desc:   "ETCD key that indicates if a cluster serves or not read traffic",
		EnvVar: "READ_ENABLED_KEY",
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

		go func() {
			serveAdminEndpoints(*appSystemCode, *appName, *port)
		}()

		keyAPI := configureETCDAPI(*etcdURL)
		monitorAnnotationsFlow(*eventReaderURL, *etcdKey, keyAPI)
		waitForSignal()
	}
	err := app.Run(os.Args)
	if err != nil {
		logger.Errorf(nil, "App could not start, error=[%s]\n", err)
		return
	}
}

func configureETCDAPI(etcdAddress string) client.KeysAPI {
	cfg := client.Config{
		Endpoints: []string{etcdAddress},
	}

	c, err := client.New(cfg)
	if err != nil {
		logger.FatalEvent("ETCD client couldn't be created: %v", err)
	}
	return client.NewKeysAPI(c)
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
