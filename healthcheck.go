package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

const (
	healthPath = "/__health"
	gtgPath    = "/__gtg"
)

type healthService struct {
	config     *healthConfig
	checks     []health.Check
	httpClient http.Client
}

type healthConfig struct {
	appSystemCode  string
	appName        string
	port           string
	eventReaderUrl string
}

func newHealthService(config *healthConfig) *healthService {
	service := &healthService{config: config}
	service.checks = []health.Check{
		service.eventReaderCheck(),
	}
	service.httpClient = http.Client{
		Timeout: time.Duration(10 * time.Second),
	}

	return service
}

func (service *healthService) eventReaderCheck() health.Check {
	return health.Check{
		BusinessImpact:   "Event reader is not available, the success of an annotation publish can't be determined.",
		Name:             "Event reader availability healthcheck",
		PanicGuide:       "https://dewey.ft.com/annotations-monitoring-service.html",
		Severity:         1,
		TechnicalSummary: "Splunk event reader is not reachable.",
		Checker:          service.eventReaderReachabilityChecker,
	}
}

func (service *healthService) eventReaderReachabilityChecker() (string, error) {

	req, err := http.NewRequest("GET", service.config.eventReaderUrl+gtgPath, nil)
	if err != nil {
		return fmt.Sprintf("Error creating requests for url=%s", req.URL.String()), err
	}

	resp, err := service.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("Error executing requests for url=%s", req.URL.String()), err
	}

	defer cleanUp(resp)

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Connecting to %s was not successful. Status: %d", req.URL.String(), resp.StatusCode), fmt.Errorf("Status: %d", resp.StatusCode)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Could not read payload from response for url=%s", req.URL.String()), err
	}

	return "Splunk event reader is healthy", nil
}

func (service *healthService) gtgCheck() gtg.Status {
	for _, check := range service.checks {
		if _, err := check.Checker(); err != nil {
			return gtg.Status{GoodToGo: false, Message: err.Error()}
		}
	}
	return gtg.Status{GoodToGo: true}
}
