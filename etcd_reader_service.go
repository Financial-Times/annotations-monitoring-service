package main

import (
	"github.com/Financial-Times/go-logger"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"strconv"
)

func readEnabled(kAPI client.KeysAPI) bool {
	//TODO: check how many successful transactions are in 5 minutes in prod, how heavily would etcd be requested... would it handle so many requests?

	//By default, the cluster will be considered as active. Consider inactive only if ETCD value confirms (failovers).
	readEnabled := true

	resp, err := kAPI.Get(context.Background(), "/ft/healthcheck-categories/read/enabled", nil)
	if err != nil {
		logger.Errorf(nil, "Couldn't determine if the cluster is active. ETCD key can't be read. Error %v", err)
		return readEnabled
	}

	b, err := strconv.ParseBool(resp.Node.Value)
	if err != nil {
		logger.Errorf(nil, "Couldn't determine if the cluster is active. ETCD key can't be parsed. Error %v", err)
		return readEnabled
	}

	return b
}
