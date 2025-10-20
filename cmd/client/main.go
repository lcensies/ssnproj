package main

import (
	"context"
	"flag"

	runner "github.com/lcensies/ssnproj/cmd/client/cmd/runner"
	"go.uber.org/zap"
)

var logger *zap.Logger

var (
	host  string
	port  string
	debug bool
)

func init() {
	flag.StringVar(&host, "host", "localhost", "host to connect to")
	flag.StringVar(&port, "port", "8080", "port to connect to")
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.Parse()

	// Configure logger based on debug flag
	if debug {
		logger, _ = zap.NewDevelopment()
	} else {
		// No-op logger (silent)
		logger = zap.NewNop()
	}
}

func main() {
	defer logger.Sync()
	ctx := context.Background()
	logger.Info("Starting the client...")
	if err := runner.RunClient(ctx, host, port, logger); err != nil {
		logger.Error("error running client", zap.Error(err))
		return
	}
	logger.Info("Client started successfully")
}
