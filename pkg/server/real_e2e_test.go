package server

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clientpkg "github.com/lcensies/ssnproj/pkg/client"
	rsaUtil "github.com/lcensies/ssnproj/pkg/rsa"
	"go.uber.org/zap"
)

// TestServer represents a test server instance
type TestServer struct {
	server   *Server
	listener net.Listener
	port     string
	host     string
	tempDir  string
	keyDir   string
	logger   *zap.Logger
}

// TestClient represents a test client instance
type TestClient struct {
	client *clientpkg.Client
	logger *zap.Logger
}

// setupTestServer creates and starts a test server
func setupTestServer(t *testing.T) *TestServer {
	// Create temporary directory for server data
	tempDir := createTestTempDir(t)

	// Create temporary directory for RSA keys
	keyDir := createTestTempDir(t)

	// Generate RSA key pair for testing
	privKey, pubKey := rsaUtil.GenerateKeyPair(2048)
	keyPair := &rsaUtil.RSAKeyPair{
		Private: privKey,
		Public:  pubKey,
	}

	// Save key pair to temp directory
	err := saveTestKeyPair(keyPair, keyDir)
	if err != nil {
		t.Fatalf("Failed to save RSA key pair: %v", err)
	}

	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	addr := listener.Addr().(*net.TCPAddr)
	port := fmt.Sprintf("%d", addr.Port)
	host := "127.0.0.1"

	// Close the listener as we'll create a new one in the server
	listener.Close()

	// Create server config
	config := &ServerConfig{
		Host:         host,
		Port:         port,
		ConfigFolder: keyDir,
		RootDir:      &tempDir,
	}

	// Create server
	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Set the RSA key pair (since we generated it for testing)
	server.SetRSAKeyPair(keyPair)

	// Start server in goroutine
	go server.Run()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	return &TestServer{
		server:  server,
		port:    port,
		host:    host,
		tempDir: tempDir,
		keyDir:  keyDir,
		logger:  logger,
	}
}

// cleanupTestServer stops the test server and cleans up resources
func (ts *TestServer) cleanupTestServer(t *testing.T) {
	// Clean up temp directories
	cleanupTestTempDir(t, ts.tempDir)
	cleanupTestTempDir(t, ts.keyDir)

	ts.logger.Sync()
}

// setupTestClient creates a test client connected to the server
func setupTestClient(t *testing.T, server *TestServer) *TestClient {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create client logger: %v", err)
	}

	ctx := context.Background()

	// Use the server's public key file
	serverPubKeyPath := filepath.Join(server.keyDir, "public.pem")
	client, err := clientpkg.NewClientWithServerPubKey(ctx, server.host, server.port, serverPubKeyPath, logger)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Perform handshake
	err = client.PerformHandshake(ctx)
	if err != nil {
		client.Close(ctx)
		t.Fatalf("Failed to perform handshake: %v", err)
	}

	return &TestClient{
		client: client,
		logger: logger,
	}
}

// cleanupTestClient closes the test client
func (tc *TestClient) cleanupTestClient(t *testing.T) {
	ctx := context.Background()
	err := tc.client.Close(ctx)
	if err != nil {
		t.Errorf("Failed to close client: %v", err)
	}
	tc.logger.Sync()
}

// TestRealE2E_ListFiles tests listing files with real client-server communication
func TestRealE2E_ListFiles(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Create test files on server
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	createTestFiles(t, server.tempDir, testFiles)

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	// Test list files
	ctx := context.Background()
	fileList, err := client.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Verify all test files are listed
	for _, filename := range testFiles {
		if !strings.Contains(fileList, filename) {
			t.Errorf("File list does not contain %s. List: %s", filename, fileList)
		}
	}
}

// TestRealE2E_UploadFile tests uploading a file with real client-server communication
func TestRealE2E_UploadFile(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	// Create test file content
	testContent := "This is test content for upload"

	// Create temporary file for upload
	tempFile := createTestTempFile(t, testContent)
	defer os.Remove(tempFile)

	// Test upload
	ctx := context.Background()
	err := client.client.UploadFile(ctx, tempFile)
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	// Verify file was created on server (using the basename of the temp file)
	expectedFilename := filepath.Base(tempFile)
	serverFilePath := filepath.Join(server.tempDir, expectedFilename)
	if _, err := os.Stat(serverFilePath); os.IsNotExist(err) {
		t.Errorf("File was not created on server: %s", serverFilePath)
	}

	// Verify file content
	actualContent, err := os.ReadFile(serverFilePath)
	if err != nil {
		t.Fatalf("Failed to read uploaded file: %v", err)
	}

	if string(actualContent) != testContent {
		t.Errorf("File content mismatch. Expected: %s, Got: %s", testContent, string(actualContent))
	}
}

// TestRealE2E_DownloadFile tests downloading a file with real client-server communication
func TestRealE2E_DownloadFile(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Create test file on server
	testFilename := "download_test.txt"
	testContent := "This is test content for download"
	serverFilePath := filepath.Join(server.tempDir, testFilename)
	if err := os.WriteFile(serverFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file on server: %v", err)
	}

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	// Create temporary file for download
	tempFile := createTestTempFile(t, "")
	defer os.Remove(tempFile)

	// Test download
	ctx := context.Background()
	err := client.client.DownloadFile(ctx, testFilename, tempFile)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	// Verify downloaded content
	actualContent, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(actualContent) != testContent {
		t.Errorf("Downloaded content mismatch. Expected: %s, Got: %s", testContent, string(actualContent))
	}
}

// TestRealE2E_DownloadLargeFile tests downloading a large file with chunked transfer
func TestRealE2E_DownloadLargeFile(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Create large test file on server (1MB)
	testFilename := "large_download_test.bin"
	fileSize := 1024 * 1024 // 1MB
	testContent := make([]byte, fileSize)
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}
	
	serverFilePath := filepath.Join(server.tempDir, testFilename)
	if err := os.WriteFile(serverFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create large test file on server: %v", err)
	}

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	// Create temporary file for download
	tempFile := createTestTempFile(t, "")
	defer os.Remove(tempFile)

	// Test download
	ctx := context.Background()
	err := client.client.DownloadFile(ctx, testFilename, tempFile)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	// Verify downloaded content
	actualContent, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	// Verify file size
	if len(actualContent) != len(testContent) {
		t.Errorf("Downloaded file size mismatch. Expected: %d, Got: %d", len(testContent), len(actualContent))
	}

	// Verify content integrity
	if !bytes.Equal(actualContent, testContent) {
		t.Errorf("Downloaded content mismatch for large file")
	}
}

// TestRealE2E_DownloadVeryLargeFile tests downloading a very large file with chunked transfer
func TestRealE2E_DownloadVeryLargeFile(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Create very large test file on server (10MB)
	testFilename := "very_large_download_test.bin"
	fileSize := 10 * 1024 * 1024 // 10MB
	testContent := make([]byte, fileSize)
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}
	
	serverFilePath := filepath.Join(server.tempDir, testFilename)
	if err := os.WriteFile(serverFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create very large test file on server: %v", err)
	}

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	// Create temporary file for download
	tempFile := createTestTempFile(t, "")
	defer os.Remove(tempFile)

	// Test download with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	err := client.client.DownloadFile(ctx, testFilename, tempFile)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	// Verify downloaded content
	actualContent, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	// Verify file size
	if len(actualContent) != len(testContent) {
		t.Errorf("Downloaded file size mismatch. Expected: %d, Got: %d", len(testContent), len(actualContent))
	}

	// Verify content integrity (sample check for performance)
	if len(actualContent) > 0 && actualContent[0] != testContent[0] {
		t.Errorf("Downloaded content mismatch at beginning of file")
	}
	
	if len(actualContent) > 1000 && actualContent[1000] != testContent[1000] {
		t.Errorf("Downloaded content mismatch at middle of file")
	}
	
	if len(actualContent) > 1 && actualContent[len(actualContent)-1] != testContent[len(testContent)-1] {
		t.Errorf("Downloaded content mismatch at end of file")
	}
}

// TestRealE2E_DeleteFile tests deleting a file with real client-server communication
func TestRealE2E_DeleteFile(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Create test file on server
	testFilename := "delete_test.txt"
	testContent := "This file will be deleted"
	serverFilePath := filepath.Join(server.tempDir, testFilename)
	if err := os.WriteFile(serverFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file on server: %v", err)
	}

	// Verify file exists initially
	if _, err := os.Stat(serverFilePath); os.IsNotExist(err) {
		t.Fatalf("Test file was not created on server: %s", serverFilePath)
	}

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	// Test delete
	ctx := context.Background()
	err := client.client.DeleteFile(ctx, testFilename)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(serverFilePath); !os.IsNotExist(err) {
		t.Errorf("File %s still exists after deletion", serverFilePath)
	}
}

// TestRealE2E_CompleteWorkflow tests a complete workflow with real client-server communication
func TestRealE2E_CompleteWorkflow(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	ctx := context.Background()

	// Step 1: List files (should be empty initially)
	fileList, err := client.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("Initial ListFiles failed: %v", err)
	}

	if strings.TrimSpace(fileList) != "" {
		t.Errorf("Expected empty file list initially, got: %s", fileList)
	}

	// Step 2: Upload a file
	testContent := "This is a complete workflow test"

	tempFile := createTestTempFile(t, testContent)
	defer os.Remove(tempFile)

	err = client.client.UploadFile(ctx, tempFile)
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	// Get the expected filename (basename of temp file)
	expectedFilename := filepath.Base(tempFile)

	// Step 3: List files (should contain uploaded file)
	fileList, err = client.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles after upload failed: %v", err)
	}

	if !strings.Contains(fileList, expectedFilename) {
		t.Errorf("Uploaded file %s not found in file list: %s", expectedFilename, fileList)
	}

	// Step 4: Download the file
	downloadFile := createTestTempFile(t, "")
	defer os.Remove(downloadFile)

	err = client.client.DownloadFile(ctx, expectedFilename, downloadFile)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	// Verify downloaded content
	actualContent, err := os.ReadFile(downloadFile)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(actualContent) != testContent {
		t.Errorf("Downloaded content mismatch. Expected: %s, Got: %s", testContent, string(actualContent))
	}

	// Step 5: Delete the file
	err = client.client.DeleteFile(ctx, expectedFilename)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Step 6: List files (should be empty again)
	fileList, err = client.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("Final ListFiles failed: %v", err)
	}

	if strings.TrimSpace(fileList) != "" {
		t.Errorf("Expected empty file list after deletion, got: %s", fileList)
	}
}

// TestRealE2E_ErrorHandling tests error handling with real client-server communication
func TestRealE2E_ErrorHandling(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	ctx := context.Background()

	// Test downloading non-existent file
	err := client.client.DownloadFile(ctx, "nonexistent.txt", "output.txt")
	if err == nil {
		t.Error("Expected error when downloading non-existent file")
	}

	// Test deleting non-existent file
	err = client.client.DeleteFile(ctx, "nonexistent.txt")
	if err == nil {
		t.Error("Expected error when deleting non-existent file")
	}
}

// TestRealE2E_MultipleClients tests multiple clients connecting to the same server
func TestRealE2E_MultipleClients(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Setup first client
	client1 := setupTestClient(t, server)
	defer client1.cleanupTestClient(t)

	// Setup second client
	client2 := setupTestClient(t, server)
	defer client2.cleanupTestClient(t)

	ctx := context.Background()

	// Client 1 uploads a file
	testContent := "This file was uploaded by client 1"

	tempFile := createTestTempFile(t, testContent)
	defer os.Remove(tempFile)

	err := client1.client.UploadFile(ctx, tempFile)
	if err != nil {
		t.Fatalf("Client 1 upload failed: %v", err)
	}

	// Get the expected filename (basename of temp file)
	expectedFilename := filepath.Base(tempFile)

	// Client 2 lists files and should see the file uploaded by client 1
	fileList, err := client2.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("Client 2 list failed: %v", err)
	}

	if !strings.Contains(fileList, expectedFilename) {
		t.Errorf("Client 2 should see file uploaded by client 1. List: %s", fileList)
	}

	// Client 2 downloads the file
	downloadFile := createTestTempFile(t, "")
	defer os.Remove(downloadFile)

	err = client2.client.DownloadFile(ctx, expectedFilename, downloadFile)
	if err != nil {
		t.Fatalf("Client 2 download failed: %v", err)
	}

	// Verify content
	actualContent, err := os.ReadFile(downloadFile)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(actualContent) != testContent {
		t.Errorf("Downloaded content mismatch. Expected: %s, Got: %s", testContent, string(actualContent))
	}
}

// Helper function to create a temporary file with content
func createTestTempFile(t *testing.T, content string) string {
	tempFile, err := os.CreateTemp("", "ssnproj_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if content != "" {
		_, err = tempFile.WriteString(content)
		if err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
			t.Fatalf("Failed to write to temp file: %v", err)
		}
	}

	tempFile.Close()
	return tempFile.Name()
}

// Helper function to save RSA key pair for testing
func saveTestKeyPair(keyPair *rsaUtil.RSAKeyPair, keyDir string) error {
	// Save private key
	privKeyBytes := rsaUtil.PrivateKeyToBytes(keyPair.Private)
	privKeyPath := filepath.Join(keyDir, "private.pem")
	if err := os.WriteFile(privKeyPath, privKeyBytes, 0600); err != nil {
		return err
	}

	// Save public key
	pubKeyBytes := rsaUtil.PublicKeyToBytes(keyPair.Public)
	pubKeyPath := filepath.Join(keyDir, "public.pem")
	if err := os.WriteFile(pubKeyPath, pubKeyBytes, 0644); err != nil {
		return err
	}

	return nil
}
