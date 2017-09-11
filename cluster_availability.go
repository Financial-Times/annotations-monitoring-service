package main

import (
	"errors"
	"github.com/domainr/dnsr"
	"strings"
)

type Cluster struct {
	readDNS string
	tag     string
}
type ClusterService interface {
	IsActive()
}

type monitoredClusterService struct {
	instance Cluster
}

func newMonitoredClusterService(readDNS string, tag string) monitoredClusterService {
	instance := Cluster{readDNS: readDNS, tag: tag}
	return monitoredClusterService{instance: instance}
}

func (mc monitoredClusterService) IsActive() (bool, error) {
	// if the read address contains the cluster tag (and implicitly the region)
	// than this means that there is no failover mechanism in place
	if strings.Contains(mc.instance.readDNS, mc.instance.tag) {
		return true, nil
	}
	resolver := dnsr.New(5)
	cNames := resolver.Resolve(mc.instance.readDNS, "CNAME")
	if len(cNames) > 0 {
		return strings.Contains(cNames[0].Value, mc.instance.tag), nil
	}
	return false, errors.New("Address could not be resolved, maybe it is invalid")
}
