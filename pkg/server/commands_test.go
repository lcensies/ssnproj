package server

import (
	"bytes"
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

func TestHandleDownload_ChunkedTransfer(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	logger := createTestLogger(t)
	defer logger.Sync()

	// Create a large test file (larger than chunk size)
	filename := "large_test_file.txt"
	fileContent := make([]byte, 200*1024) // 200KB file
	for i := range fileContent {
		fileContent[i] = byte(i % 256)
	}
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

	// Verify initial response was sent
	if len(mockConn.sentMessages) < 1 {
		t.Fatalf("Expected at least 1 sent message, got %d", len(mockConn.sentMessages))
	}

	// Check initial response
	initialResponse := mockConn.sentMessages[0]
	if initialResponse.Type != protocol.MessageTypeResponse {
		t.Errorf("Expected initial response type %v, got %v", protocol.MessageTypeResponse, initialResponse.Type)
	}

	respMsg, err := protocol.DeserializeResponse(initialResponse.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize initial response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected initial success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Verify chunks were sent
	chunkCount := 0
	for i := 1; i < len(mockConn.sentMessages); i++ {
		msg := mockConn.sentMessages[i]
		if msg.Type == protocol.MessageTypeData {
			chunkCount++
			
			// Deserialize chunk data
			chunk, err := protocol.DeserializeChunkData(msg.Payload)
			if err != nil {
				t.Fatalf("Failed to deserialize chunk %d: %v", chunkCount, err)
			}

			// Verify chunk metadata
			if chunk.Filename != filename {
				t.Errorf("Chunk %d filename mismatch: expected %s, got %s", chunkCount, filename, chunk.Filename)
			}

			if chunk.ChunkIndex != uint32(chunkCount-1) {
				t.Errorf("Chunk %d index mismatch: expected %d, got %d", chunkCount, chunkCount-1, chunk.ChunkIndex)
			}

			// Verify chunk data integrity
			expectedStart := chunk.ChunkIndex * 64 * 1024 // 64KB chunks
			expectedEnd := expectedStart + uint32(len(chunk.Data))
			if expectedEnd > uint32(len(fileContent)) {
				expectedEnd = uint32(len(fileContent))
			}
			expectedData := fileContent[expectedStart:expectedEnd]

			if !bytes.Equal(chunk.Data, expectedData) {
				t.Errorf("Chunk %d data mismatch", chunkCount)
			}
		}
	}

	// Verify we got multiple chunks for a large file
	if chunkCount < 2 {
		t.Errorf("Expected multiple chunks for large file, got %d", chunkCount)
	}

	// Verify total chunks calculation
	expectedChunks := uint32((len(fileContent) + 64*1024 - 1) / (64 * 1024))
	if chunkCount != int(expectedChunks) {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, chunkCount)
	}
}

func TestSendFileInChunks_SmallFile(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	logger := createTestLogger(t)
	defer logger.Sync()

	// Create a small test file (smaller than chunk size)
	filename := "small_test_file.txt"
	fileContent := []byte("This is a small file")
	
	// Create mock connection handler
	mockConn := &MockConnectionHandler{}
	cmdHandler := NewCommandHandler(mockConn, logger, &tempDir)

	// Test sendFileInChunks directly
	err := cmdHandler.sendFileInChunks(filename, fileContent)
	if err != nil {
		t.Fatalf("sendFileInChunks failed: %v", err)
	}

	// Verify exactly one chunk was sent
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 chunk for small file, got %d", len(mockConn.sentMessages))
	}

	// Verify chunk data
	chunkMsg := mockConn.sentMessages[0]
	if chunkMsg.Type != protocol.MessageTypeData {
		t.Errorf("Expected chunk message type %v, got %v", protocol.MessageTypeData, chunkMsg.Type)
	}

	chunk, err := protocol.DeserializeChunkData(chunkMsg.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize chunk: %v", err)
	}

	// Verify chunk metadata
	if chunk.Filename != filename {
		t.Errorf("Chunk filename mismatch: expected %s, got %s", filename, chunk.Filename)
	}

	if chunk.ChunkIndex != 0 {
		t.Errorf("Expected chunk index 0, got %d", chunk.ChunkIndex)
	}

	if chunk.TotalChunks != 1 {
		t.Errorf("Expected total chunks 1, got %d", chunk.TotalChunks)
	}

	if chunk.TotalSize != uint64(len(fileContent)) {
		t.Errorf("Expected total size %d, got %d", len(fileContent), chunk.TotalSize)
	}

	if !bytes.Equal(chunk.Data, fileContent) {
		t.Errorf("Chunk data mismatch")
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
