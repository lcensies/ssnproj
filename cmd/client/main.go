package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	runner "github.com/lcensies/ssnproj/cmd/client/cmd/runner"
	"go.uber.org/zap"
)

var logger *zap.Logger

var (
	host            string
	port            string
	debug           bool
	serverPubKeyPem string
)

func init() {
	var err error
	// Load .env file if it exists (optional, won't fail if missing)
	_ = godotenv.Load()

	// Get environment variables
	serverPubKeyPem = os.Getenv("SERVER_PUBLIC_KEY")
	// Define flags with environment variables as defaults
	flag.StringVar(&host, "host", "localhost", "host to connect to")
	flag.StringVar(&port, "port", "8080", "port to connect to")
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.Parse()

	logger, err = zap.NewProduction()
	if err != nil {
		fmt.Println("Failed to create logger", err)
		os.Exit(1)
	}
}

func main() {
	defer logger.Sync()
	ctx := context.Background()
	if serverPubKeyPem == "" {
		logger.Error("server public key path is not set")
		return
	}
	rsaPubKey, err := parsePEM([]byte(serverPubKeyPem))
	if err != nil {
		logger.Error("failed to parse server public key", zap.Error(err))
		return
	}
	logger.Info("Starting the client...")
	if err := runner.RunClient(ctx, host, port, rsaPubKey, logger); err != nil {
		logger.Error("error running client", zap.Error(err))
		return
	}
	logger.Info("Client started successfully")
}

func parsePEM(pemKey []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing public key: block is nil")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DER encoded public key: %w", err)
	}
	rsaKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("provided key is not an RSA public key")
	}
	return rsaKey, nil
}
