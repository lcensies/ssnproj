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

	responsePayload, err := protocol.SerializeResponse(true, "File downloaded successfully", fileData)
	if err != nil {
		return err
	}

	response := protocol.NewMessage(protocol.MessageTypeResponse, responsePayload)
	return handler.conn.SendSecureMessage(response)
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
