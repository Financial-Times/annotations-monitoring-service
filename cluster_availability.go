package main

import (
	"errors"
	"github.com/domainr/dnsr"
	"strings"
)

func isClusterActive(readDNS string, environmentTag string) (bool, error) {
	// if the read address contains the cluster tag (and implicitly the region)
	// than this means that there is no failover mechanism in place
	if strings.Contains(readDNS, environmentTag) {
		return true, nil
	}
	resolver := dnsr.New(5)
	cNames := resolver.Resolve(readDNS, "CNAME")
	if len(cNames) > 1 {
		return strings.Contains(cNames[0].Value, environmentTag), nil
	}
	return false, errors.New("Address could not be resolved, maybe it is invalid")
}
