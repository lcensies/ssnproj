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

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	ctx := context.Background()

	// Upload test files through the client so they go to the correct client directory
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, filename := range testFiles {
		// Create temp file with test content
		tempFile := createTestTempFile(t, "test content for "+filename)
		defer os.Remove(tempFile)

		// Upload it
		err := client.client.UploadFile(ctx, tempFile)
		if err != nil {
			t.Fatalf("Failed to upload %s: %v", filename, err)
		}
	}

	// Test list files
	fileList, err := client.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Verify we got some files back (uploaded files will have temp names)
	if fileList == "" {
		t.Errorf("File list is empty, expected uploaded files")
	}

	// Verify we have the correct number of files
	fileCount := len(strings.Split(strings.TrimSpace(fileList), "\n"))
	if fileList != "" && fileCount != len(testFiles) {
		t.Errorf("Expected %d files, got %d. List: %s", len(testFiles), fileCount, fileList)
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

	// Verify by listing files
	fileList, err := client.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	expectedFilename := filepath.Base(tempFile)
	if !strings.Contains(fileList, expectedFilename) {
		t.Errorf("Uploaded file not found in list. Expected: %s, List: %s", expectedFilename, fileList)
	}

	// Verify by downloading the file back
	downloadFile := createTestTempFile(t, "")
	defer os.Remove(downloadFile)

	err = client.client.DownloadFile(ctx, expectedFilename, downloadFile)
	if err != nil {
		t.Fatalf("Failed to download uploaded file: %v", err)
	}

	actualContent, err := os.ReadFile(downloadFile)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
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

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	ctx := context.Background()

	// First upload a file
	testContent := "This is test content for download"
	uploadFile := createTestTempFile(t, testContent)
	defer os.Remove(uploadFile)

	err := client.client.UploadFile(ctx, uploadFile)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	testFilename := filepath.Base(uploadFile)

	// Create temporary file for download
	downloadFile := createTestTempFile(t, "")
	defer os.Remove(downloadFile)

	// Test download
	err = client.client.DownloadFile(ctx, testFilename, downloadFile)
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
}

// TestRealE2E_DownloadLargeFile tests downloading a large file with chunked transfer
func TestRealE2E_DownloadLargeFile(t *testing.T) {
	// Setup server
	server := setupTestServer(t)
	defer server.cleanupTestServer(t)

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	ctx := context.Background()

	// Create large test file (1MB)
	fileSize := 1024 * 1024 // 1MB
	testContent := make([]byte, fileSize)
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}

	// Upload the large file
	uploadFile := createTestTempFile(t, "")
	defer os.Remove(uploadFile)
	if err := os.WriteFile(uploadFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	err := client.client.UploadFile(ctx, uploadFile)
	if err != nil {
		t.Fatalf("Failed to upload large test file: %v", err)
	}

	testFilename := filepath.Base(uploadFile)

	// Create temporary file for download
	downloadFile := createTestTempFile(t, "")
	defer os.Remove(downloadFile)

	// Test download
	err = client.client.DownloadFile(ctx, testFilename, downloadFile)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	// Verify downloaded content
	actualContent, err := os.ReadFile(downloadFile)
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

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	// Create very large test file (10MB)
	fileSize := 10 * 1024 * 1024 // 10MB
	testContent := make([]byte, fileSize)
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}

	// Upload the very large file
	uploadFile := createTestTempFile(t, "")
	defer os.Remove(uploadFile)
	if err := os.WriteFile(uploadFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create very large test file: %v", err)
	}

	// Test upload with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := client.client.UploadFile(ctx, uploadFile)
	if err != nil {
		t.Fatalf("Failed to upload very large test file: %v", err)
	}

	testFilename := filepath.Base(uploadFile)

	// Create temporary file for download
	downloadFile := createTestTempFile(t, "")
	defer os.Remove(downloadFile)

	// Test download with timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	err = client.client.DownloadFile(ctx2, testFilename, downloadFile)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	// Verify downloaded content
	actualContent, err := os.ReadFile(downloadFile)
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

	// Setup client
	client := setupTestClient(t, server)
	defer client.cleanupTestClient(t)

	ctx := context.Background()

	// First upload a file
	testContent := "This file will be deleted"
	uploadFile := createTestTempFile(t, testContent)
	defer os.Remove(uploadFile)

	err := client.client.UploadFile(ctx, uploadFile)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	testFilename := filepath.Base(uploadFile)

	// Verify it's there
	fileList, err := client.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if !strings.Contains(fileList, testFilename) {
		t.Fatalf("File not found after upload: %s", testFilename)
	}

	// Test delete
	err = client.client.DeleteFile(ctx, testFilename)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	// Verify file was deleted by checking list
	fileList, err = client.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles failed after delete: %v", err)
	}
	if strings.Contains(fileList, testFilename) {
		t.Errorf("File still exists after deletion: %s", testFilename)
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

	// Client 2 lists files - should NOT see client 1's files (isolated storage)
	fileList, err := client2.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("Client 2 list failed: %v", err)
	}

	// Verify isolation: Client 2 should NOT see Client 1's files
	if strings.Contains(fileList, expectedFilename) {
		t.Errorf("Client 2 should NOT see file uploaded by client 1 (isolated storage). List: %s", fileList)
	}

	// Client 2 attempts to download client 1's file - should fail
	downloadFile := createTestTempFile(t, "")
	defer os.Remove(downloadFile)

	err = client2.client.DownloadFile(ctx, expectedFilename, downloadFile)
	if err == nil {
		t.Errorf("Client 2 should NOT be able to download client 1's file (isolated storage)")
	}

	// Verify that client 1 can still access its own file
	client1List, err := client1.client.ListFiles(ctx)
	if err != nil {
		t.Fatalf("Client 1 list failed: %v", err)
	}

	if !strings.Contains(client1List, expectedFilename) {
		t.Errorf("Client 1 should see its own uploaded file. List: %s", client1List)
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
