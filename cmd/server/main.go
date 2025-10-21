package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lcensies/ssnproj/pkg/server"
	"go.uber.org/zap"
)

const (
	// Default values
	defaultHost         = "localhost"
	defaultPort         = "8080"
	defaultConfigFolder = "configs/server"
	defaultRootDir      = "data"
)

// Config holds the server configuration
type Config struct {
	Host         string
	Port         string
	ConfigFolder string
	RootDir      string
	LogLevel     string
}

// loadConfig loads configuration from environment variables and command-line flags
func loadConfig() *Config {
	config := &Config{}

	// Define command-line flags
	host := flag.String("host", getEnvOrDefault("SERVER_HOST", defaultHost), "Server host address")
	port := flag.String("port", getEnvOrDefault("SERVER_PORT", defaultPort), "Server port")
	configFolder := flag.String("config", getEnvOrDefault("SERVER_CONFIG_FOLDER", defaultConfigFolder), "Configuration folder path")
	rootDir := flag.String("root-dir", getEnvOrDefault("SERVER_ROOT_DIR", defaultRootDir), "Root directory for file operations")
	logLevel := flag.String("log-level", getEnvOrDefault("SERVER_LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")

	// Parse command-line flags
	flag.Parse()

	// Set configuration values
	config.Host = *host
	config.Port = *port
	config.ConfigFolder = *configFolder
	config.RootDir = *rootDir
	config.LogLevel = *logLevel

	return config
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// createLogger creates a logger based on the log level
func createLogger(logLevel string) (*zap.Logger, error) {
	var config zap.Config

	switch logLevel {
	case "debug":
		config = zap.NewDevelopmentConfig()
	case "info", "warn", "error":
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		if logLevel == "warn" {
			config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
		}
		if logLevel == "error" {
			config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
		}
	default:
		config = zap.NewProductionConfig()
	}

	return config.Build()
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if config.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	if config.ConfigFolder == "" {
		return fmt.Errorf("config folder cannot be empty")
	}
	if config.RootDir == "" {
		return fmt.Errorf("root directory cannot be empty")
	}
	return nil
}

// printConfig prints the current configuration
func printConfig(config *Config, logger *zap.Logger) {
	logger.Info("Server configuration",
		zap.String("host", config.Host),
		zap.String("port", config.Port),
		zap.String("config_folder", config.ConfigFolder),
		zap.String("root_dir", config.RootDir),
		zap.String("log_level", config.LogLevel),
	)
}

// printUsage prints usage information
func printUsage() {
	fmt.Println("Secure File Transfer Server")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  go run cmd/server/main.go [flags]")
	fmt.Println("")
	fmt.Println("Flags:")
	fmt.Println("  -host string")
	fmt.Println("        Server host address (default: localhost)")
	fmt.Println("        Environment variable: SERVER_HOST")
	fmt.Println("")
	fmt.Println("  -port string")
	fmt.Println("        Server port (default: 8080)")
	fmt.Println("        Environment variable: SERVER_PORT")
	fmt.Println("")
	fmt.Println("  -config string")
	fmt.Println("        Configuration folder path (default: configs/server)")
	fmt.Println("        Environment variable: SERVER_CONFIG_FOLDER")
	fmt.Println("")
	fmt.Println("  -root-dir string")
	fmt.Println("        Root directory for file operations (default: data)")
	fmt.Println("        Environment variable: SERVER_ROOT_DIR")
	fmt.Println("")
	fmt.Println("  -log-level string")
	fmt.Println("        Log level: debug, info, warn, error (default: info)")
	fmt.Println("        Environment variable: SERVER_LOG_LEVEL")
	fmt.Println("")
	fmt.Println("  -help")
	fmt.Println("        Show this help message")
	fmt.Println("")
	fmt.Println("Environment Variables:")
	fmt.Println("  SERVER_HOST         - Server host address")
	fmt.Println("  SERVER_PORT         - Server port")
	fmt.Println("  SERVER_CONFIG_FOLDER - Configuration folder path")
	fmt.Println("  SERVER_ROOT_DIR     - Root directory for file operations")
	fmt.Println("  SERVER_LOG_LEVEL    - Log level")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  # Run with default settings")
	fmt.Println("  go run cmd/server/main.go")
	fmt.Println("")
	fmt.Println("  # Run with custom port")
	fmt.Println("  go run cmd/server/main.go -port 9000")
	fmt.Println("")
	fmt.Println("  # Run with environment variables")
	fmt.Println("  SERVER_PORT=9000 SERVER_LOG_LEVEL=debug go run cmd/server/main.go")
	fmt.Println("")
	fmt.Println("  # Run with custom config and data directories")
	fmt.Println("  go run cmd/server/main.go -config /etc/ssnproj -root-dir /var/lib/ssnproj")
}

func main() {
	// Check for help flag
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help") {
		printUsage()
		return
	}

	// Load configuration
	config := loadConfig()

	// Create logger
	logger, err := createLogger(config.LogLevel)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Validate configuration
	if err := validateConfig(config); err != nil {
		logger.Fatal("Configuration validation failed", zap.Error(err))
	}

	// Print configuration
	printConfig(config, logger)

	// Create server configuration
	serverConfig := &server.ServerConfig{
		Host:         config.Host,
		Port:         config.Port,
		ConfigFolder: config.ConfigFolder,
		RootDir:      &config.RootDir,
		Logger:       logger,
	}

	// Create server
	srv, err := server.NewServer(serverConfig)
	if err != nil {
		logger.Fatal("Failed to create server", zap.Error(err))
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		logger.Info("Starting server",
			zap.String("address", fmt.Sprintf("%s:%s", config.Host, config.Port)))
		srv.Run()
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	logger.Info("Shutting down server...")

	// Here you could add cleanup logic if needed
	// For now, the server will stop when the main goroutine exits
}
