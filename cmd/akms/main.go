package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/prometheus/client_golang/prometheus"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alpe/akms/pkg/signer"
	"github.com/alpe/akms/pkg/store"

	"github.com/alpe/akms/pkg/server"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
)

const (
	envNodeID = "NODE_ID"
	envNodeIP = "NODE_IP"
)

func main() {
	var (
		chainID     = flag.String("chain-id", "", "network chain address")
		serverPort  = flag.Int("server-port", 8080, "server port")
		filePath    = flag.String("file-path", "", "file path to load/store keyfile")
		snapshotDir = flag.String("snapshot-dir", "", "file path to load/store snapshots")
		raftPort    = flag.Int("raft-port", 8088, "raft bind addr port")
		raftPeers   = flag.String("raft-peers", "", "raft peer addresses=id@address,id2@address2")
		logLevel    = flag.String("log-level", "info", "values: debug, info, error, none")
		//signTimeout = flag.Duration("sign-timeout", 5*time.Millisecond, "state deadline")
	)
	flag.Parse()
	ll, err := log.AllowLevel(*logLevel)
	if err != nil {
		ll = log.AllowInfo()
	}
	logger := log.NewFilter(log.NewTMLogger(log.NewSyncWriter(os.Stdout)), ll)

	if len(*chainID) == 0 {
		logger.Error("chain-id must not be empty")
		os.Exit(1)
	}
	if len(*filePath) == 0 {
		logger.Error("state-file must not be empty")
		os.Exit(1)
	}
	if len(*snapshotDir) == 0 {
		logger.Error("snapshot-dir must not be empty")
		os.Exit(1)
	}
	if len(*raftPeers) == 0 {
		logger.Error("raft-peers must not be empty")
		os.Exit(1)
	}
	peers := strings.Split(*raftPeers, ",")
	if len(peers) == 0 {
		logger.Error("raft-peers must not be empty")
		os.Exit(1)
	}
	nodeID := os.Getenv(envNodeID)
	if len(nodeID) == 0 {
		logger.Error("NODE_ID environment variable not be empty")
		os.Exit(1)
	}
	nodeIP := os.Getenv(envNodeIP)
	if len(nodeIP) == 0 {
		var err error
		if nodeIP, err = ipAddress(); err != nil {
			logger.Error("ip address not found", "cause", err)
			os.Exit(1)
		}
	}

	defer recoverToLog(logger)
	var state *privval.FilePV

	switch _, err := os.Stat(*filePath); {
	case os.IsNotExist(err):
		// copy from seed file
		orig := privval.LoadFilePV(*filePath+".orig", *filePath+".orig")
		state = privval.GenFilePV(*filePath, *filePath+".state")
		privKey := orig.Key.PrivKey
		state.Key.Address = privKey.PubKey().Address()
		state.Key.PubKey = privKey.PubKey()
		state.Key.PrivKey = privKey
		state.Save()
	case err == nil:
		// todo: revisit if we need to preserve state....
		state = privval.LoadFilePVEmptyState(*filePath, *filePath+".state")
	default:
		logger.Error("state-file could not be loaded", "cause", err)
		os.Exit(1)
	}
	promRegistry := prometheus.NewRegistry()
	raftStore := store.NewRaftStore(*snapshotDir, fmt.Sprintf("%s:%d", nodeIP, *raftPort), logger)
	store.RegisterRaftStoreMetrics(promRegistry, raftStore, logger)
	raftServers := make([]raft.Server, len(peers))
	for i, v := range peers {
		tokens := strings.Split(v, "@")
		if len(tokens) != 2 {
			logger.Error("raft-peers must match ip@addrss", "source", v)
			os.Exit(1)
		}
		raftServers[i] = raft.Server{ID: raft.ServerID(tokens[0]), Address: raft.ServerAddress(tokens[1])}
	}
	if err := raftStore.Open(nodeID, raftServers...); err != nil {
		logger.Error("Failed to start raft node", "cause", err)
		os.Exit(1)

	}
	privVal := signer.New(state.Key.PrivKey, raftStore, logger)
	httpPort := *serverPort
	srv := server.NewJsonApiServer(httpPort, logger, privVal, *chainID, promRegistry)

	trap(context.Background(), logger, func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("Http server crashed", "cause", err)
		}
	})

	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	srv.Shutdown(ctx)
	ctx, _ = context.WithTimeout(context.Background(), 2*time.Second)
	raftStore.Shutdown(ctx)
	logger.Info("Server stopped")
}

func trap(ctx context.Context, logger log.Logger, callback func()) {
	ctx, cancel := context.WithCancel(ctx)
	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, os.Interrupt, syscall.SIGTERM)
	go func() {
		callback()
		cancel()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-gracefulStop:
			logger.Info("Interrupt signal received, shutting down")
			cancel()
		}
	}
}

func recoverToLog(logger log.Logger) {
	if err := recover(); err != nil {
		logger.Error("Recover from panic", "cause", err)
		os.Exit(1)
	}
}

func ipAddress() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", errors.New("not found")
}
