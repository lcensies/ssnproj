package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/lcensies/ssnproj/pkg/protocol"
	"go.uber.org/zap"
)

// MockConnectionHandler is a mock implementation for testing
type MockConnectionHandler struct {
	sentMessages []*protocol.Message
}

func (c *MockConnectionHandler) SendSecureMessage(message *protocol.Message) error {
	// Store the message for testing
	c.sentMessages = append(c.sentMessages, message)
	return nil
}

func (c *MockConnectionHandler) GetSentMessages() []*protocol.Message {
	return c.sentMessages
}

func (c *MockConnectionHandler) ClearSentMessages() {
	c.sentMessages = make([]*protocol.Message, 0)
}

// Test helper functions
func createTestTempDir(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "ssnproj_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return tempDir
}

func cleanupTestTempDir(t *testing.T, tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		t.Errorf("Failed to cleanup temp dir: %v", err)
	}
}

func createTestFiles(t *testing.T, dir string, filenames []string) {
	for _, filename := range filenames {
		filePath := filepath.Join(dir, filename)
		content := fmt.Sprintf("Test content for %s", filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}
}

func createTestLogger(t *testing.T) *zap.Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return logger
}

func TestHandleList(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	logger := createTestLogger(t)
	defer logger.Sync()

	// Create test files
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	createTestFiles(t, tempDir, testFiles)

	// Create mock connection handler
	mockConn := &MockConnectionHandler{}
	cmdHandler := NewCommandHandler(mockConn, logger, &tempDir)

	// Test handleList
	command := &protocol.CommandMessage{
		Command:  protocol.CommandList,
		Filename: "",
		Data:     nil,
	}

	err := cmdHandler.handleList(command)
	if err != nil {
		t.Fatalf("handleList failed: %v", err)
	}

	// Verify response was sent
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	if response.Type != protocol.MessageTypeResponse {
		t.Errorf("Expected response type %v, got %v", protocol.MessageTypeResponse, response.Type)
	}

	// Deserialize response
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v", respMsg.Success)
	}

	// Check that all test files are listed
	fileList := respMsg.Message
	for _, filename := range testFiles {
		if !strings.Contains(fileList, filename) {
			t.Errorf("File list does not contain %s. List: %s", filename, fileList)
		}
	}
}

func TestHandleUpload(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	logger := createTestLogger(t)
	defer logger.Sync()

	// Create mock connection handler
	mockConn := &MockConnectionHandler{}
	cmdHandler := NewCommandHandler(mockConn, logger, &tempDir)

	// Test data
	filename := "test_upload.txt"
	fileContent := []byte("This is test content for upload")

	command := &protocol.CommandMessage{
		Command:  protocol.CommandUpload,
		Filename: filename,
		Data:     fileContent,
	}

	err := cmdHandler.handleUpload(command)
	if err != nil {
		t.Fatalf("handleUpload failed: %v", err)
	}

	// Verify response was sent
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	if response.Type != protocol.MessageTypeResponse {
		t.Errorf("Expected response type %v, got %v", protocol.MessageTypeResponse, response.Type)
	}

	// Deserialize response
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Verify file was created
	filePath := filepath.Join(tempDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("File was not created: %s", filePath)
	}

	// Verify file content
	actualContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read uploaded file: %v", err)
	}

	if string(actualContent) != string(fileContent) {
		t.Errorf("File content mismatch. Expected: %s, Got: %s", string(fileContent), string(actualContent))
	}
}

func TestHandleDownload(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	logger := createTestLogger(t)
	defer logger.Sync()

	// Create test file
	filename := "test_download.txt"
	fileContent := []byte("This is test content for download")
	filePath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(filePath, fileContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create mock connection handler
	mockConn := &MockConnectionHandler{}
	cmdHandler := NewCommandHandler(mockConn, logger, &tempDir)

	command := &protocol.CommandMessage{
		Command:  protocol.CommandDownload,
		Filename: filename,
		Data:     nil,
	}

	err := cmdHandler.handleDownload(command)
	if err != nil {
		t.Fatalf("handleDownload failed: %v", err)
	}

	// Verify response was sent
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	if response.Type != protocol.MessageTypeResponse {
		t.Errorf("Expected response type %v, got %v", protocol.MessageTypeResponse, response.Type)
	}

	// Deserialize response
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Verify file content in response
	if string(respMsg.Data) != string(fileContent) {
		t.Errorf("Downloaded content mismatch. Expected: %s, Got: %s", string(fileContent), string(respMsg.Data))
	}
}

func TestHandleDownload_FileNotFound(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	logger := createTestLogger(t)
	defer logger.Sync()

	// Create mock connection handler
	mockConn := &MockConnectionHandler{}
	cmdHandler := NewCommandHandler(mockConn, logger, &tempDir)

	command := &protocol.CommandMessage{
		Command:  protocol.CommandDownload,
		Filename: "nonexistent.txt",
		Data:     nil,
	}

	err := cmdHandler.handleDownload(command)
	if err != nil {
		t.Fatalf("handleDownload failed: %v", err)
	}

	// Verify response was sent
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if respMsg.Success {
		t.Errorf("Expected success=false for nonexistent file, got %v", respMsg.Success)
	}
}

func TestHandleDelete(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	logger := createTestLogger(t)
	defer logger.Sync()

	// Create test file
	filename := "test_delete.txt"
	fileContent := []byte("This file will be deleted")
	filePath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(filePath, fileContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Test file was not created: %s", filePath)
	}

	// Create mock connection handler
	mockConn := &MockConnectionHandler{}
	cmdHandler := NewCommandHandler(mockConn, logger, &tempDir)

	command := &protocol.CommandMessage{
		Command:  protocol.CommandDelete,
		Filename: filename,
		Data:     nil,
	}

	err := cmdHandler.handleDelete(command)
	if err != nil {
		t.Fatalf("handleDelete failed: %v", err)
	}

	// Verify response was sent
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	if response.Type != protocol.MessageTypeResponse {
		t.Errorf("Expected response type %v, got %v", protocol.MessageTypeResponse, response.Type)
	}

	// Deserialize response
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Verify file was deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("File was not deleted: %s", filePath)
	}
}

func TestHandleDelete_FileNotFound(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	logger := createTestLogger(t)
	defer logger.Sync()

	// Create mock connection handler
	mockConn := &MockConnectionHandler{}
	cmdHandler := NewCommandHandler(mockConn, logger, &tempDir)

	command := &protocol.CommandMessage{
		Command:  protocol.CommandDelete,
		Filename: "nonexistent.txt",
		Data:     nil,
	}

	err := cmdHandler.handleDelete(command)
	if err != nil {
		t.Fatalf("handleDelete failed: %v", err)
	}

	// Verify response was sent
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if respMsg.Success {
		t.Errorf("Expected success=false for nonexistent file, got %v", respMsg.Success)
	}
}
