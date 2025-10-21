package server

import (
	"fmt"
	"os"
	"strings"

	protocol "github.com/lcensies/ssnproj/pkg/protocol"
	"go.uber.org/zap"
)

// ConnectionSender interface for sending secure messages
type ConnectionSender interface {
	SendSecureMessage(message *protocol.Message) error
}

type CommandHandler struct {
	conn    ConnectionSender
	logger  *zap.Logger
	rootDir *string
}

func NewCommandHandler(conn ConnectionSender, logger *zap.Logger, rootDirectory *string) *CommandHandler {
	return &CommandHandler{
		conn:    conn,
		logger:  logger,
		rootDir: rootDirectory,
	}
}

func (handler *CommandHandler) handleUpload(command *protocol.CommandMessage) error {
	handler.logger.Info("Upload command received", zap.String("filename", command.Filename))

	clientDir, err := handler.getClientDir()
	if err != nil {
		responsePayload, _ := protocol.SerializeResponse(false, "Failed to get client directory", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return err
	}

	// Create the file path
	filePath := clientDir + "/" + command.Filename

	// Write the file data
	err = os.WriteFile(filePath, command.Data, 0644)
	if err != nil {
		responsePayload, _ := protocol.SerializeResponse(false, "Failed to write file", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return err
	}

	responsePayload, err := protocol.SerializeResponse(true, "File uploaded successfully", nil)
	if err != nil {
		return err
	}

	response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
	return handler.conn.SendSecureMessage(response)
}

func (handler *CommandHandler) handleDownload(command *protocol.CommandMessage) error {
	handler.logger.Info("Download command received", zap.String("filename", command.Filename))

	clientDir, err := handler.getClientDir()
	if err != nil {
		responsePayload, _ := protocol.SerializeResponse(false, "Failed to get client directory", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return err
	}

	// Create the file path
	filePath := clientDir + "/" + command.Filename

	// Read the file data
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		responsePayload, _ := protocol.SerializeResponse(false, "File not found or failed to read", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return nil // Don't return the error, we've sent a response
	}

	// Send initial response indicating chunked transfer will begin
	responsePayload, err := protocol.SerializeResponse(true, "Starting chunked download", nil)
	if err != nil {
		return err
	}

	response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
	if err := handler.conn.SendSecureMessage(response); err != nil {
		return err
	}

	// Send file in chunks
	return handler.sendFileInChunks(command.Filename, fileData)
}

// sendFileInChunks sends a file in chunks with progress information
func (handler *CommandHandler) sendFileInChunks(filename string, fileData []byte) error {
	const chunkSize = 64 * 1024 // 64KB chunks
	totalSize := uint64(len(fileData))
	totalChunks := uint32((totalSize + chunkSize - 1) / chunkSize) // Round up division

	handler.logger.Info("Sending file in chunks",
		zap.String("filename", filename),
		zap.Uint64("totalSize", totalSize),
		zap.Uint32("totalChunks", totalChunks),
		zap.Uint32("chunkSize", chunkSize))

	for i := uint32(0); i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > uint32(totalSize) {
			end = uint32(totalSize)
		}

		chunkData := fileData[start:end]
		actualChunkSize := uint32(len(chunkData))

		// Create chunk message
		chunk := &protocol.ChunkDataMessage{
			Filename:    filename,
			ChunkIndex:  i,
			TotalChunks: totalChunks,
			ChunkSize:   actualChunkSize,
			TotalSize:   totalSize,
			Data:        chunkData,
		}

		// Serialize chunk
		chunkPayload, err := protocol.SerializeChunkData(chunk)
		if err != nil {
			return fmt.Errorf("failed to serialize chunk %d: %w", i, err)
		}

		// Send chunk as data message
		chunkMsg := protocol.NewMessage(protocol.MessageTypeData, chunkPayload)
		if err := handler.conn.SendSecureMessage(chunkMsg); err != nil {
			return fmt.Errorf("failed to send chunk %d: %w", i, err)
		}

		// Log progress
		progress := float64(i+1) / float64(totalChunks) * 100
		handler.logger.Debug("Sent chunk",
			zap.String("filename", filename),
			zap.Uint32("chunkIndex", i),
			zap.Float64("progress", progress))
	}

	handler.logger.Info("File transfer completed", zap.String("filename", filename))
	return nil
}

func (handler *CommandHandler) getClientDir() (string, error) {
	return *handler.rootDir, nil
}

func (handler *CommandHandler) handleList(command *protocol.CommandMessage) error {
	clientDir, err := handler.getClientDir()
	if err != nil {
		responsePayload, _ := protocol.SerializeResponse(false, "Failed to get client directory", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return err
	}

	handler.logger.Info("List command received", zap.String("filename", command.Filename))
	files, err := os.ReadDir(clientDir)
	if err != nil {
		responsePayload, _ := protocol.SerializeResponse(false, "Failed to read directory", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return err
	}

	filenames := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() { // Only include files, not directories
			filenames = append(filenames, file.Name())
		}
	}

	fileList := strings.Join(filenames, "\n")
	responsePayload, err := protocol.SerializeResponse(true, fileList, nil)
	if err != nil {
		return err
	}

	response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
	return handler.conn.SendSecureMessage(response)
}

func (handler *CommandHandler) handleDelete(command *protocol.CommandMessage) error {
	handler.logger.Info("Delete command received", zap.String("filename", command.Filename))

	clientDir, err := handler.getClientDir()
	if err != nil {
		responsePayload, _ := protocol.SerializeResponse(false, "Failed to get client directory", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return err
	}

	// Create the file path
	filePath := clientDir + "/" + command.Filename

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		responsePayload, _ := protocol.SerializeResponse(false, "File not found", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return nil // Don't return the error, we've sent a response
	}

	// Delete the file
	err = os.Remove(filePath)
	if err != nil {
		responsePayload, _ := protocol.SerializeResponse(false, "Failed to delete file", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return err
	}

	responsePayload, err := protocol.SerializeResponse(true, "File deleted successfully", nil)
	if err != nil {
		return err
	}

	response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
	return handler.conn.SendSecureMessage(response)
}

func (handler *CommandHandler) handle(command *protocol.CommandMessage) error {
	handler.logger.Info("Command message received", zap.String("command", string(command.Command)))
	switch command.Command {
	case protocol.CommandUpload:
		return handler.handleUpload(command)
	case protocol.CommandDownload:
		return handler.handleDownload(command)
	case protocol.CommandList:
		return handler.handleList(command)
	case protocol.CommandDelete:
		return handler.handleDelete(command)
	default:
		responsePayload, _ := protocol.SerializeResponse(false, "Unknown command", nil)
		response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
		handler.conn.SendSecureMessage(response)
		return fmt.Errorf("unknown command: %v", command.Command)
	}
}
