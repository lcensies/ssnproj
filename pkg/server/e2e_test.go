package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/lcensies/ssnproj/pkg/protocol"
	"go.uber.org/zap"
)

// E2E test helper functions - simplified to test handlers directly
func createTestCommandHandler(t *testing.T, tempDir string) (*CommandHandler, *MockConnectionHandler) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	mockConn := &MockConnectionHandler{}
	// Generate a test AES key for the handler
	testAESKey := make([]byte, 32) // 256-bit key
	cmdHandler := NewCommandHandler(mockConn, logger, &tempDir, testAESKey)

	return cmdHandler, mockConn
}

func TestE2E_ListFiles(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	// Create command handler
	cmdHandler, mockConn := createTestCommandHandler(t, tempDir)

	// Get client directory
	clientDir, err := cmdHandler.getClientDir()
	if err != nil {
		t.Fatalf("Failed to get client directory: %v", err)
	}

	// Create test files in client directory
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	createTestFiles(t, clientDir, testFiles)

	// Test list files
	command := &protocol.CommandMessage{
		Command:  protocol.CommandList,
		Filename: "",
		Data:     nil,
	}

	err = cmdHandler.handleList(command)
	if err != nil {
		t.Fatalf("handleList failed: %v", err)
	}

	// Verify response
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v", respMsg.Success)
	}

	// Verify all test files are listed
	fileList := respMsg.Message
	for _, filename := range testFiles {
		if !strings.Contains(fileList, filename) {
			t.Errorf("File list does not contain %s. List: %s", filename, fileList)
		}
	}
}

func TestE2E_UploadFile(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	// Create command handler
	cmdHandler, mockConn := createTestCommandHandler(t, tempDir)

	// Test upload
	testFilename := "upload_test.txt"
	testContent := "This is test content for upload"

	command := &protocol.CommandMessage{
		Command:  protocol.CommandUpload,
		Filename: testFilename,
		Data:     []byte(testContent),
	}

	err := cmdHandler.handleUpload(command)
	if err != nil {
		t.Fatalf("handleUpload failed: %v", err)
	}

	// Verify response
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Get client directory
	clientDir, err := cmdHandler.getClientDir()
	if err != nil {
		t.Fatalf("Failed to get client directory: %v", err)
	}

	// Verify file was created in client directory
	filePath := filepath.Join(clientDir, testFilename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("File was not created: %s", filePath)
	}

	// Verify file content
	actualContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read uploaded file: %v", err)
	}

	if string(actualContent) != testContent {
		t.Errorf("File content mismatch. Expected: %s, Got: %s", testContent, string(actualContent))
	}
}

func TestE2E_DownloadFile(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	// Create command handler
	cmdHandler, mockConn := createTestCommandHandler(t, tempDir)

	// Get client directory
	clientDir, err := cmdHandler.getClientDir()
	if err != nil {
		t.Fatalf("Failed to get client directory: %v", err)
	}

	// Create test file in client directory
	testFilename := "download_test.txt"
	testContent := "This is test content for download"
	filePath := filepath.Join(clientDir, testFilename)
	if err := os.WriteFile(filePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test download
	command := &protocol.CommandMessage{
		Command:  protocol.CommandDownload,
		Filename: testFilename,
		Data:     nil,
	}

	err = cmdHandler.handleDownload(command)
	if err != nil {
		t.Fatalf("handleDownload failed: %v", err)
	}

	// Verify response (now includes both response and chunk messages)
	if len(mockConn.sentMessages) < 2 {
		t.Fatalf("Expected at least 2 sent messages (response + chunk), got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Verify chunk data
	chunkMsg := mockConn.sentMessages[1]
	chunk, err := protocol.DeserializeChunkData(chunkMsg.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize chunk: %v", err)
	}

	if string(chunk.Data) != testContent {
		t.Errorf("Downloaded content mismatch. Expected: %s, Got: %s", testContent, string(chunk.Data))
	}
}

func TestE2E_DeleteFile(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	// Create command handler
	cmdHandler, mockConn := createTestCommandHandler(t, tempDir)

	// Get client directory
	clientDir, err := cmdHandler.getClientDir()
	if err != nil {
		t.Fatalf("Failed to get client directory: %v", err)
	}

	// Create test file in client directory
	testFilename := "delete_test.txt"
	testContent := "This file will be deleted"
	filePath := filepath.Join(clientDir, testFilename)
	if err := os.WriteFile(filePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify file exists initially
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Test file was not created: %s", filePath)
	}

	// Test delete
	command := &protocol.CommandMessage{
		Command:  protocol.CommandDelete,
		Filename: testFilename,
		Data:     nil,
	}

	err = cmdHandler.handleDelete(command)
	if err != nil {
		t.Fatalf("handleDelete failed: %v", err)
	}

	// Verify response
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Verify file was deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("File %s still exists after deletion", filePath)
	}
}

func TestE2E_CompleteWorkflow(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	// Create command handler
	cmdHandler, mockConn := createTestCommandHandler(t, tempDir)

	// Step 1: List files (should be empty initially)
	listCmd := &protocol.CommandMessage{
		Command:  protocol.CommandList,
		Filename: "",
		Data:     nil,
	}

	err := cmdHandler.handleList(listCmd)
	if err != nil {
		t.Fatalf("Initial handleList failed: %v", err)
	}

	// Verify empty list
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v", respMsg.Success)
	}

	if strings.TrimSpace(respMsg.Message) != "" {
		t.Errorf("Expected empty file list initially, got: %s", respMsg.Message)
	}

	// Clear messages for next test
	mockConn.ClearSentMessages()

	// Step 2: Upload a file
	testFilename := "workflow_test.txt"
	testContent := "This is a complete workflow test"

	uploadCmd := &protocol.CommandMessage{
		Command:  protocol.CommandUpload,
		Filename: testFilename,
		Data:     []byte(testContent),
	}

	err = cmdHandler.handleUpload(uploadCmd)
	if err != nil {
		t.Fatalf("handleUpload failed: %v", err)
	}

	// Verify upload success
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response = mockConn.sentMessages[0]
	respMsg, err = protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Clear messages for next test
	mockConn.ClearSentMessages()

	// Step 3: List files (should contain uploaded file)
	err = cmdHandler.handleList(listCmd)
	if err != nil {
		t.Fatalf("ListFiles after upload failed: %v", err)
	}

	response = mockConn.sentMessages[0]
	respMsg, err = protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !strings.Contains(respMsg.Message, testFilename) {
		t.Errorf("Uploaded file %s not found in file list: %s", testFilename, respMsg.Message)
	}

	// Clear messages for next test
	mockConn.ClearSentMessages()

	// Step 4: Download the file
	downloadCmd := &protocol.CommandMessage{
		Command:  protocol.CommandDownload,
		Filename: testFilename,
		Data:     nil,
	}

	err = cmdHandler.handleDownload(downloadCmd)
	if err != nil {
		t.Fatalf("handleDownload failed: %v", err)
	}

	// Verify response and chunk
	if len(mockConn.sentMessages) < 2 {
		t.Fatalf("Expected at least 2 sent messages (response + chunk), got %d", len(mockConn.sentMessages))
	}

	response = mockConn.sentMessages[0]
	respMsg, err = protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Verify downloaded content from chunk
	chunkMsg := mockConn.sentMessages[1]
	chunk, err := protocol.DeserializeChunkData(chunkMsg.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize chunk: %v", err)
	}

	if string(chunk.Data) != testContent {
		t.Errorf("Downloaded content mismatch. Expected: %s, Got: %s", testContent, string(chunk.Data))
	}

	// Clear messages for next test
	mockConn.ClearSentMessages()

	// Step 5: Delete the file
	deleteCmd := &protocol.CommandMessage{
		Command:  protocol.CommandDelete,
		Filename: testFilename,
		Data:     nil,
	}

	err = cmdHandler.handleDelete(deleteCmd)
	if err != nil {
		t.Fatalf("handleDelete failed: %v", err)
	}

	response = mockConn.sentMessages[0]
	respMsg, err = protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if !respMsg.Success {
		t.Errorf("Expected success=true, got %v. Message: %s", respMsg.Success, respMsg.Message)
	}

	// Clear messages for next test
	mockConn.ClearSentMessages()

	// Step 6: List files (should be empty again)
	err = cmdHandler.handleList(listCmd)
	if err != nil {
		t.Fatalf("Final handleList failed: %v", err)
	}

	response = mockConn.sentMessages[0]
	respMsg, err = protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if strings.TrimSpace(respMsg.Message) != "" {
		t.Errorf("Expected empty file list after deletion, got: %s", respMsg.Message)
	}
}

func TestE2E_ErrorHandling(t *testing.T) {
	// Setup
	tempDir := createTestTempDir(t)
	defer cleanupTestTempDir(t, tempDir)

	// Create command handler
	cmdHandler, mockConn := createTestCommandHandler(t, tempDir)

	// Test downloading non-existent file
	downloadCmd := &protocol.CommandMessage{
		Command:  protocol.CommandDownload,
		Filename: "nonexistent.txt",
		Data:     nil,
	}

	err := cmdHandler.handleDownload(downloadCmd)
	if err != nil {
		t.Fatalf("handleDownload failed: %v", err)
	}

	// Verify error response
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response := mockConn.sentMessages[0]
	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if respMsg.Success {
		t.Error("Expected success=false when downloading non-existent file")
	}

	// Clear messages for next test
	mockConn.ClearSentMessages()

	// Test deleting non-existent file
	deleteCmd := &protocol.CommandMessage{
		Command:  protocol.CommandDelete,
		Filename: "nonexistent.txt",
		Data:     nil,
	}

	err = cmdHandler.handleDelete(deleteCmd)
	if err != nil {
		t.Fatalf("handleDelete failed: %v", err)
	}

	// Verify error response
	if len(mockConn.sentMessages) != 1 {
		t.Fatalf("Expected 1 sent message, got %d", len(mockConn.sentMessages))
	}

	response = mockConn.sentMessages[0]
	respMsg, err = protocol.DeserializeResponse(response.Payload)
	if err != nil {
		t.Fatalf("Failed to deserialize response: %v", err)
	}

	if respMsg.Success {
		t.Error("Expected success=false when deleting non-existent file")
	}
}
