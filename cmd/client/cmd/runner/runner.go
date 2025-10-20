package client

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcensies/ssnproj/cmd/client/internal/entity"
	"go.uber.org/zap"
)

// RunClient starts the client and connects to the server
func RunClient(ctx context.Context, host string, port string, logger *zap.Logger) error {
	// Create client and connect
	client, err := entity.NewClient(ctx, host, port, logger)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close(ctx)

	logger.Info("Connected to server", zap.String("host", host), zap.String("port", port))

	// Perform RSA handshake
	if err := client.PerformHandshake(ctx); err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	logger.Info("Handshake completed successfully")

	// Start interactive CLI
	return runInteractiveCLI(ctx, client, logger)
}

func runInteractiveCLI(ctx context.Context, client *entity.Client, logger *zap.Logger) error {
	reader := bufio.NewReader(os.Stdin)

	printHelp()

	for {
		select {
		case <-ctx.Done():
			logger.Info("context done, stopping client")
			return nil
		default:
			if err := processCommand(ctx, client, logger, reader); err != nil {
				if err.Error() == "exit" {
					return nil
				}
				return err
			}
		}
	}
}

func processCommand(ctx context.Context, client *entity.Client, logger *zap.Logger, reader *bufio.Reader) error {
	fmt.Print("\n> ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	parts := strings.Fields(input)
	command := strings.ToLower(parts[0])

	switch command {
	case "help", "h":
		printHelp()
	case "upload", "up":
		handleUpload(ctx, client, logger, parts)
	case "download", "dl":
		handleDownload(ctx, client, logger, parts)
	case "list", "ls":
		handleList(ctx, client, logger)
	case "delete", "del", "rm":
		handleDelete(ctx, client, logger, parts, reader)
	case "exit", "quit", "q":
		fmt.Println("Goodbye!")
		return fmt.Errorf("exit")
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Type 'help' for available commands")
	}
	return nil
}

func handleUpload(ctx context.Context, client *entity.Client, logger *zap.Logger, parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: upload <filename>")
		return
	}
	filename := parts[1]
	if err := client.UploadFile(ctx, filename); err != nil {
		fmt.Printf("Error uploading file: %v\n", err)
		logger.Error("upload failed", zap.Error(err))
	} else {
		fmt.Printf("✓ File '%s' uploaded successfully\n", filename)
	}
}

func handleDownload(ctx context.Context, client *entity.Client, logger *zap.Logger, parts []string) {
	if len(parts) < 2 {
		fmt.Println("Usage: download <filename> [output_path]")
		return
	}
	filename := parts[1]
	outputPath := filename
	if len(parts) >= 3 {
		outputPath = parts[2]
	} else {
		// Save to current directory with same name
		outputPath = filepath.Base(filename)
	}

	if err := client.DownloadFile(ctx, filename, outputPath); err != nil {
		fmt.Printf("Error downloading file: %v\n", err)
		logger.Error("download failed", zap.Error(err))
	} else {
		fmt.Printf("✓ File downloaded to '%s'\n", outputPath)
	}
}

func handleList(ctx context.Context, client *entity.Client, logger *zap.Logger) {
	fileList, err := client.ListFiles(ctx)
	if err != nil {
		fmt.Printf("Error listing files: %v\n", err)
		logger.Error("list failed", zap.Error(err))
		return
	}
	fmt.Println("\nFiles on server:")
	fmt.Println("================")
	if fileList == "" {
		fmt.Println("(no files)")
	} else {
		fmt.Println(fileList)
	}
}

func handleDelete(ctx context.Context, client *entity.Client, logger *zap.Logger, parts []string, reader *bufio.Reader) {
	if len(parts) < 2 {
		fmt.Println("Usage: delete <filename>")
		return
	}
	filename := parts[1]

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete '%s'? (y/n): ", filename)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "y" && confirm != "yes" {
		fmt.Println("Delete cancelled")
		return
	}

	if err := client.DeleteFile(ctx, filename); err != nil {
		fmt.Printf("Error deleting file: %v\n", err)
		logger.Error("delete failed", zap.Error(err))
	} else {
		fmt.Printf("✓ File '%s' deleted successfully\n", filename)
	}
}

func printHelp() {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          Secure File Transfer Client - Commands             ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  upload <filename>              Upload a file to the server")
	fmt.Println("  download <filename> [output]   Download a file from the server")
	fmt.Println("  list                           List all files on the server")
	fmt.Println("  delete <filename>              Delete a file from the server")
	fmt.Println("  help                           Show this help message")
	fmt.Println("  exit                           Disconnect and exit")
	fmt.Println()
	fmt.Println("Aliases:")
	fmt.Println("  up = upload  |  dl = download  |  ls = list  |  rm/del = delete")
	fmt.Println()
}
