package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	aesUtil "github.com/lcensies/ssnproj/pkg/aes"
	entity "github.com/lcensies/ssnproj/pkg/client"
	rsaUtil "github.com/lcensies/ssnproj/pkg/rsa"
	"go.uber.org/zap"
)

// Benchmark file sizes
const (
	smallFileSize  = 1024 * 10          // 10 KB
	mediumFileSize = 1024 * 1024        // 1 MB
	largeFileSize  = 1024 * 1024 * 10   // 10 MB
	hugeFileSize   = 1024 * 1024 * 1024 // 1 GB
)

// setupBenchmarkServer creates a server for benchmarking
func setupBenchmarkServer(b *testing.B) (*Server, *string, func()) {
	// Create temp directory for test data
	tempDir, err := os.MkdirTemp("", "bench_server_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create temp directory for keys
	keyDir, err := os.MkdirTemp("", "bench_keys_*")
	if err != nil {
		os.RemoveAll(tempDir)
		b.Fatalf("Failed to create key dir: %v", err)
	}

	// Generate RSA key pair
	privKey, pubKey := rsaUtil.GenerateKeyPair(2048)
	keyPair := &rsaUtil.RSAKeyPair{
		Private: privKey,
		Public:  pubKey,
	}

	// Save keys
	privKeyBytes := rsaUtil.PrivateKeyToBytes(privKey)
	pubKeyBytes := rsaUtil.PublicKeyToBytes(pubKey)
	os.WriteFile(filepath.Join(keyDir, "private.pem"), privKeyBytes, 0600)
	os.WriteFile(filepath.Join(keyDir, "public.pem"), pubKeyBytes, 0644)

	// Create logger (nop for benchmarks)
	logger := zap.NewNop()

	// Create server config
	config := &ServerConfig{
		Host:         "localhost",
		Port:         "0", // Random port
		ConfigFolder: keyDir,
		RootDir:      &tempDir,
		Logger:       logger,
	}

	// Create server
	server, err := NewServer(config)
	if err != nil {
		os.RemoveAll(tempDir)
		os.RemoveAll(keyDir)
		b.Fatalf("Failed to create server: %v", err)
	}

	server.SetRSAKeyPair(keyPair)

	cleanup := func() {
		os.RemoveAll(tempDir)
		os.RemoveAll(keyDir)
	}

	return server, &tempDir, cleanup
}

// generateRandomData creates random data of specified size
func generateRandomData(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

// BenchmarkAESEncryption tests AES encryption performance
func BenchmarkAESEncryption(b *testing.B) {
	key, _ := aesUtil.GenerateKey()
	data := generateRandomData(mediumFileSize)

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		_, err := aesUtil.Encrypt(data, key)
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}
	}
}

// BenchmarkAESDecryption tests AES decryption performance
func BenchmarkAESDecryption(b *testing.B) {
	key, _ := aesUtil.GenerateKey()
	data := generateRandomData(mediumFileSize)
	encrypted, _ := aesUtil.Encrypt(data, key)

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		_, err := aesUtil.Decrypt(encrypted, key)
		if err != nil {
			b.Fatalf("Decryption failed: %v", err)
		}
	}
}

// BenchmarkRSAKeyPairGeneration tests RSA key generation
func BenchmarkRSAKeyPairGeneration(b *testing.B) {
	for i := 0; i < b.N; i++ {
		rsaUtil.GenerateKeyPair(2048)
	}
}

// BenchmarkRSAEncryption tests RSA encryption (for AES key exchange)
func BenchmarkRSAEncryption(b *testing.B) {
	privKey, pubKey := rsaUtil.GenerateKeyPair(2048)
	aesKey, _ := aesUtil.GenerateKey()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		encrypted := rsaUtil.EncryptWithPublicKey(aesKey, pubKey)
		_ = rsaUtil.DecryptWithPrivateKey(encrypted, privKey)
	}
}

// BenchmarkFileUpload benchmarks file upload operation
func BenchmarkFileUpload(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"Small_10KB", smallFileSize},
		{"Medium_1MB", mediumFileSize},
		{"Large_10MB", largeFileSize},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			server, rootDir, cleanup := setupBenchmarkServer(b)
			defer cleanup()

			// Start server
			go server.Run()

			// Create test file
			testData := generateRandomData(size.size)
			testFile := filepath.Join(os.TempDir(), fmt.Sprintf("bench_upload_%d.bin", size.size))
			os.WriteFile(testFile, testData, 0644)
			defer os.Remove(testFile)

			b.ResetTimer()
			b.SetBytes(int64(size.size))

			// Note: This is a simplified benchmark
			// In real scenario, we would need to setup client connection
			// For now, we measure the directory creation and file operations
			for i := 0; i < b.N; i++ {
				clientDir := filepath.Join(*rootDir, fmt.Sprintf("client_%d", i))
				os.MkdirAll(clientDir, 0755)
				targetFile := filepath.Join(clientDir, "test.bin")
				os.WriteFile(targetFile, testData, 0644)
			}
		})
	}
}

// BenchmarkFileDownload benchmarks file download operation
func BenchmarkFileDownload(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"Small_10KB", smallFileSize},
		{"Medium_1MB", mediumFileSize},
		{"Large_10MB", largeFileSize},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			_, rootDir, cleanup := setupBenchmarkServer(b)
			defer cleanup()

			// Create test file
			testData := generateRandomData(size.size)
			clientDir := filepath.Join(*rootDir, "test_client")
			os.MkdirAll(clientDir, 0755)
			testFile := filepath.Join(clientDir, "test.bin")
			os.WriteFile(testFile, testData, 0644)

			b.ResetTimer()
			b.SetBytes(int64(size.size))

			for i := 0; i < b.N; i++ {
				_, err := os.ReadFile(testFile)
				if err != nil {
					b.Fatalf("Read failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkChunking benchmarks the chunking algorithm
func BenchmarkChunking(b *testing.B) {
	chunkSizes := []struct {
		name string
		size uint32
	}{
		{"16KB", 16 * 1024},
		{"32KB", 32 * 1024},
		{"64KB", 64 * 1024},
		{"128KB", 128 * 1024},
		{"256KB", 256 * 1024},
		{"512KB", 512 * 1024},
		{"1MB", 1024 * 1024},
	}

	fileData := generateRandomData(largeFileSize)

	for _, cs := range chunkSizes {
		b.Run(cs.name, func(b *testing.B) {
			b.ResetTimer()
			b.SetBytes(int64(len(fileData)))

			for i := 0; i < b.N; i++ {
				chunkSize := cs.size
				totalChunks := uint32((uint64(len(fileData)) + uint64(chunkSize) - 1) / uint64(chunkSize))

				for j := uint32(0); j < totalChunks; j++ {
					start := j * chunkSize
					end := start + chunkSize
					if end > uint32(len(fileData)) {
						end = uint32(len(fileData))
					}
					_ = fileData[start:end]
				}
			}
		})
	}
}

// BenchmarkClientDirCreation benchmarks client directory creation based on AES key
func BenchmarkClientDirCreation(b *testing.B) {
	_, rootDir, cleanup := setupBenchmarkServer(b)
	defer cleanup()

	logger := zap.NewNop()
	aesKey, _ := aesUtil.GenerateKey()

	handler := &CommandHandler{
		logger:  logger,
		rootDir: rootDir,
		aesKey:  aesKey,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := handler.getClientDir()
		if err != nil {
			b.Fatalf("Failed to get client dir: %v", err)
		}
	}
}

// BenchmarkConcurrentClients simulates multiple clients
func BenchmarkConcurrentClients(b *testing.B) {
	_, rootDir, cleanup := setupBenchmarkServer(b)
	defer cleanup()

	logger := zap.NewNop()
	testData := generateRandomData(mediumFileSize)

	b.ResetTimer()
	b.SetBytes(int64(len(testData)))

	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets unique AES key (simulating different client)
		aesKey, _ := aesUtil.GenerateKey()
		handler := &CommandHandler{
			logger:  logger,
			rootDir: rootDir,
			aesKey:  aesKey,
		}

		for pb.Next() {
			clientDir, err := handler.getClientDir()
			if err != nil {
				b.Fatalf("Failed to get client dir: %v", err)
			}
			testFile := filepath.Join(clientDir, "test.bin")
			os.WriteFile(testFile, testData, 0644)
		}
	})
}

// BenchmarkFullUploadDownloadCycle benchmarks complete upload-download cycle
func BenchmarkFullUploadDownloadCycle(b *testing.B) {
	ctx := context.Background()

	sizes := []struct {
		name string
		size int
	}{
		{"Small_10KB", smallFileSize},
		{"Medium_1MB", mediumFileSize},
		{"Large_10MB", largeFileSize},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			server, rootDir, cleanup := setupBenchmarkServer(b)
			defer cleanup()

			// Start server in background
			listener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				b.Fatalf("Failed to create listener: %v", err)
			}
			defer listener.Close()

			// Get the actual port
			_, port, err := net.SplitHostPort(listener.Addr().String())
			if err != nil {
				b.Fatalf("Failed to get port: %v", err)
			}

			// Start server in goroutine
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return // Listener closed
					}
					client := NewConnectionHandler(conn, server.rsaKeyPair, server.logger, rootDir)
					go client.HandleRawRequest()
				}
			}()

			// Create test file
			testData := generateRandomData(size.size)
			testFile := filepath.Join(os.TempDir(), fmt.Sprintf("bench_upload_%d.bin", size.size))
			os.WriteFile(testFile, testData, 0644)
			defer os.Remove(testFile)

			// Create output file path
			outputFile := filepath.Join(os.TempDir(), fmt.Sprintf("bench_download_%d.bin", size.size))
			defer os.Remove(outputFile)

			// Create public key file for client
			pubKeyFile := filepath.Join(os.TempDir(), "server_public.pem")
			pubKeyBytes := rsaUtil.PublicKeyToBytes(server.rsaKeyPair.Public)
			os.WriteFile(pubKeyFile, pubKeyBytes, 0644)
			defer os.Remove(pubKeyFile)

			b.ResetTimer()
			b.SetBytes(int64(size.size))

			for i := 0; i < b.N; i++ {
				// Create client with server's public key file
				client, err := entity.NewClientWithServerPubKey(ctx, "localhost", port, pubKeyFile, zap.NewNop())
				if err != nil {
					b.Fatalf("Failed to create client: %v", err)
				}

				// Perform handshake
				if err := client.PerformHandshake(ctx); err != nil {
					client.Close(ctx)
					b.Fatalf("Handshake failed: %v", err)
				}

				// Upload file
				if err := client.UploadFile(ctx, testFile); err != nil {
					client.Close(ctx)
					b.Fatalf("Upload failed: %v", err)
				}

				// Download file
				if err := client.DownloadFile(ctx, filepath.Base(testFile), outputFile); err != nil {
					client.Close(ctx)
					b.Fatalf("Download failed: %v", err)
				}

				// Verify file integrity
				downloadedData, err := os.ReadFile(outputFile)
				if err != nil {
					client.Close(ctx)
					b.Fatalf("Failed to read downloaded file: %v", err)
				}

				if len(downloadedData) != len(testData) {
					client.Close(ctx)
					b.Fatalf("File size mismatch: expected %d, got %d", len(testData), len(downloadedData))
				}

				// Close client
				client.Close(ctx)
			}
		})
	}
}

// BenchmarkLargeFileWithChunkSizes benchmarks large file transfer with different chunk sizes
func BenchmarkLargeFileWithChunkSizes(b *testing.B) {
	ctx := context.Background()

	chunkSizes := []struct {
		name string
		size uint32
	}{
		{"16KB", 16 * 1024},
		{"32KB", 32 * 1024},
		{"64KB", 64 * 1024},
		{"128KB", 128 * 1024},
		{"256KB", 256 * 1024},
		{"512KB", 512 * 1024},
		{"1MB", 1024 * 1024},
		{"2MB", 2 * 1024 * 1024},
		{"4MB", 4 * 1024 * 1024},
	}

	for _, chunkSize := range chunkSizes {
		b.Run(fmt.Sprintf("100MB_%s", chunkSize.name), func(b *testing.B) {
			server, rootDir, cleanup := setupBenchmarkServer(b)
			defer cleanup()

			// Start server in background
			listener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				b.Fatalf("Failed to create listener: %v", err)
			}
			defer listener.Close()

			// Get the actual port
			_, port, err := net.SplitHostPort(listener.Addr().String())
			if err != nil {
				b.Fatalf("Failed to get port: %v", err)
			}

			// Start server in goroutine
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return // Listener closed
					}
					client := NewConnectionHandler(conn, server.rsaKeyPair, server.logger, rootDir)
					go client.HandleRawRequest()
				}
			}()

			// Create test file (100MB)
			testFileSize := 100 * 1024 * 1024 // 100MB
			testData := generateRandomData(testFileSize)
			testFile := filepath.Join(os.TempDir(), fmt.Sprintf("bench_huge_%s.bin", chunkSize.name))
			os.WriteFile(testFile, testData, 0644)
			defer os.Remove(testFile)

			// Create output file path
			outputFile := filepath.Join(os.TempDir(), fmt.Sprintf("bench_download_%s.bin", chunkSize.name))
			defer os.Remove(outputFile)

			// Create public key file for client
			pubKeyFile := filepath.Join(os.TempDir(), "server_public.pem")
			pubKeyBytes := rsaUtil.PublicKeyToBytes(server.rsaKeyPair.Public)
			os.WriteFile(pubKeyFile, pubKeyBytes, 0644)
			defer os.Remove(pubKeyFile)

			b.ResetTimer()
			b.SetBytes(int64(testFileSize))

			// Create client with server's public key file
			client, err := entity.NewClientWithServerPubKey(ctx, "localhost", port, pubKeyFile, zap.NewNop())
			if err != nil {
				b.Fatalf("Failed to create client: %v", err)
			}

			// Perform handshake
			if err := client.PerformHandshake(ctx); err != nil {
				client.Close(ctx)
				b.Fatalf("Handshake failed: %v", err)
			}

			// Upload file
			start := time.Now()
			if err := client.UploadFile(ctx, testFile); err != nil {
				client.Close(ctx)
				b.Fatalf("Upload failed: %v", err)
			}
			uploadTime := time.Since(start)

			// Download file
			start = time.Now()
			if err := client.DownloadFile(ctx, filepath.Base(testFile), outputFile); err != nil {
				client.Close(ctx)
				b.Fatalf("Download failed: %v", err)
			}
			downloadTime := time.Since(start)

			// Verify file integrity
			downloadedData, err := os.ReadFile(outputFile)
			if err != nil {
				client.Close(ctx)
				b.Fatalf("Failed to read downloaded file: %v", err)
			}

			if len(downloadedData) != len(testData) {
				client.Close(ctx)
				b.Fatalf("File size mismatch: expected %d, got %d", len(testData), len(downloadedData))
			}

			// Close client
			client.Close(ctx)

			// Log performance metrics
			totalTime := uploadTime + downloadTime
			throughput := float64(testFileSize) / totalTime.Seconds() / (1024 * 1024) // MB/s

			b.Logf("Chunk size: %s, Upload: %v, Download: %v, Total: %v, Throughput: %.2f MB/s",
				chunkSize.name, uploadTime, downloadTime, totalTime, throughput)
		})
	}
}

// BenchmarkLargeFileUploadOnly benchmarks only upload performance for large files
func BenchmarkLargeFileUploadOnly(b *testing.B) {
	ctx := context.Background()

	fileSizes := []struct {
		name string
		size int
	}{
		{"100MB", 100 * 1024 * 1024},
		{"500MB", 500 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
	}

	for _, fileSize := range fileSizes {
		b.Run(fileSize.name, func(b *testing.B) {
			server, rootDir, cleanup := setupBenchmarkServer(b)
			defer cleanup()

			// Start server in background
			listener, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				b.Fatalf("Failed to create listener: %v", err)
			}
			defer listener.Close()

			// Get the actual port
			_, port, err := net.SplitHostPort(listener.Addr().String())
			if err != nil {
				b.Fatalf("Failed to get port: %v", err)
			}

			// Start server in goroutine
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return // Listener closed
					}
					client := NewConnectionHandler(conn, server.rsaKeyPair, server.logger, rootDir)
					go client.HandleRawRequest()
				}
			}()

			// Create test file
			testData := generateRandomData(fileSize.size)
			testFile := filepath.Join(os.TempDir(), fmt.Sprintf("bench_upload_%s.bin", fileSize.name))
			os.WriteFile(testFile, testData, 0644)
			defer os.Remove(testFile)

			// Create public key file for client
			pubKeyFile := filepath.Join(os.TempDir(), "server_public.pem")
			pubKeyBytes := rsaUtil.PublicKeyToBytes(server.rsaKeyPair.Public)
			os.WriteFile(pubKeyFile, pubKeyBytes, 0644)
			defer os.Remove(pubKeyFile)

			b.ResetTimer()
			b.SetBytes(int64(fileSize.size))

			// Create client with server's public key file
			client, err := entity.NewClientWithServerPubKey(ctx, "localhost", port, pubKeyFile, zap.NewNop())
			if err != nil {
				b.Fatalf("Failed to create client: %v", err)
			}

			// Perform handshake
			if err := client.PerformHandshake(ctx); err != nil {
				client.Close(ctx)
				b.Fatalf("Handshake failed: %v", err)
			}

			// Upload file
			start := time.Now()
			if err := client.UploadFile(ctx, testFile); err != nil {
				client.Close(ctx)
				b.Fatalf("Upload failed: %v", err)
			}
			uploadTime := time.Since(start)

			// Close client
			client.Close(ctx)

			// Log performance metrics
			throughput := float64(fileSize.size) / uploadTime.Seconds() / (1024 * 1024) // MB/s

			b.Logf("File size: %s, Upload time: %v, Throughput: %.2f MB/s",
				fileSize.name, uploadTime, throughput)
		})
	}
}
