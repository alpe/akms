package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/privval"

	"github.com/alpe/akms/pkg/signer"
	"github.com/tendermint/tendermint/libs/log"
)

const socketPrefix = "unix://"

func main() {
	var (
		chainID = flag.String("chain-id", "", "network chain address")
		// todo: tendermint server address
		socketAddr = flag.String("socket-addr", "", "server address: `unix://`")
		// todo: rename serverAddr to kmsAddr
		serverAddr = flag.String("server-addr", "", "Remote kms address: `http://`")
		// todo: revisit if signTimeout makes sense with waitTimeout on remote call set
		//signTimeout = flag.Duration("sign-timeout", 3*time.Second, "total deadline per call")
		waitTimeout = flag.Duration("wait-timeout", 500*time.Millisecond, "max time to wait for remote server resp")
	)
	flag.Parse()
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	if len(*chainID) == 0 {
		logger.Error("chain-id must not be empty")
		os.Exit(1)
	}
	if len(*socketAddr) == 0 {
		logger.Error("socket-addr must not be empty")
		os.Exit(1)
	}
	if len(*serverAddr) == 0 {
		logger.Error("server-addr must not be empty")
		os.Exit(1)
	}
	if strings.HasPrefix(strings.ToLower(*socketAddr), socketPrefix) {
		s := *socketAddr
		s = s[len(socketPrefix):]
		socketAddr = &s
	}
	defer recoverToLog(logger)

	kmsClient := &signer.Debugger{Logger: logger, Next: signer.NewRestClient(
		logger,
		&http.Client{Timeout: *waitTimeout},
		*serverAddr),
	}

	tendermintIO := privval.NewSignerServiceEndpoint(logger, *chainID, kmsClient, privval.DialUnixFn(*socketAddr))
	logger.Info("Server started", "address", *socketAddr, "chainID", *chainID, "serverAddress", *serverAddr)
	runServer(context.Background(), logger, tendermintIO)
	logger.Info("Server stopped")
}

func runServer(ctx context.Context, logger log.Logger, rs common.Service) {
	defer rs.Stop()
	trap(ctx, logger, func() {
		if err := rs.Start(); err != nil {
			logger.Error("Server terminated", "cause", err)
			return
		}
		<-rs.Quit()
	})
}

func trap(ctx context.Context, logger log.Logger, callback func()) {
	ctx, cancel := context.WithCancel(ctx)
	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, os.Interrupt, syscall.SIGTERM)
	go func() {
		callback()
		cancel()
	}()
	func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-gracefulStop:
				logger.Info("Interrupt signal received, shutting down")
				cancel()
			}
		}
	}()
}

func recoverToLog(logger log.Logger) {
	if err := recover(); err != nil {
		logger.Error("Recover from panic", "cause", err)
		os.Exit(1)
	}
}
