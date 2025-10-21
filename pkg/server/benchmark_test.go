package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	aesUtil "github.com/lcensies/ssnproj/pkg/aes"
	entity "github.com/lcensies/ssnproj/pkg/client"
	rsaUtil "github.com/lcensies/ssnproj/pkg/rsa"
	"go.uber.org/zap"
)

// Benchmark file sizes
const (
	smallFileSize  = 1024 * 10        // 10 KB
	mediumFileSize = 1024 * 1024      // 1 MB
	largeFileSize  = 1024 * 1024 * 10 // 10 MB
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

	// This is a placeholder - full E2E benchmark would require running server
	// and connecting real client
	b.Skip("Full E2E benchmark requires running server instance")

	// Setup would include:
	// 1. Start server
	// 2. Create client with proper handshake
	// 3. Upload file
	// 4. Download file
	// 5. Verify integrity

	_ = ctx
	_ = entity.NewClient
}
