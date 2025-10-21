package entity

import (
	"context"
	"crypto/rsa"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	aesutil "github.com/lcensies/ssnproj/pkg/aes"
	"github.com/lcensies/ssnproj/pkg/protocol"
	rsautil "github.com/lcensies/ssnproj/pkg/rsa"
	"go.uber.org/zap"
)

// Constants
const (
	// MaxPayloadSize is the maximum allowed payload size (4 GB)
	// This prevents memory exhaustion attacks
	MaxPayloadSize = (4 * 1024 * 1024 * 1024) - 1

	// DefaultReadTimeout is the default timeout for read operations
	DefaultReadTimeout = 30 * time.Second
)

// Error message constants
const (
	errSerializeCommand    = "failed to serialize command: %w"
	errReceiveResponse     = "failed to receive response: %w"
	errUnexpectedResponse  = "unexpected response type: %v"
	errDeserializeResponse = "failed to deserialize response: %w"
)

// Client represents the client connection to the server
type Client struct {
	conn         net.Conn
	logger       *zap.Logger
	serverPubKey *rsa.PublicKey
	aesKey       []byte
}

// NewClient creates a new client
func NewClient(ctx context.Context, host string, port string, serverPubKey *rsa.PublicKey, logger *zap.Logger) (*Client, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		conn:         conn,
		logger:       logger,
		serverPubKey: serverPubKey,
	}, nil
}

// NewClientWithServerPubKey creates a new client with server's public key loaded from file
func NewClientWithServerPubKey(ctx context.Context, host string, port string, serverPubKeyPath string, logger *zap.Logger) (*Client, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	// Load server's public key from file
	serverPubKeyBytes, err := os.ReadFile(serverPubKeyPath)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read server public key: %w", err)
	}

	serverPubKey := rsautil.BytesToPublicKey(serverPubKeyBytes)

	return &Client{
		conn:         conn,
		logger:       logger,
		serverPubKey: serverPubKey,
	}, nil
}

// Close closes the client connection
func (c *Client) Close(ctx context.Context) error {
	if c.conn != nil {
		err := c.conn.Close()
		if err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}
	return nil
}

// SendMessage sends a protocol message
func (c *Client) SendMessage(msg *protocol.Message) error {
	data, err := msg.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	_, err = c.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// ReceiveMessage receives a protocol message (unencrypted - used for handshake only)
func (c *Client) ReceiveMessage() (*protocol.Message, error) {
	// Read header (1 byte type + 4 bytes length)
	header := make([]byte, 5)
	_, err := io.ReadFull(c.conn, header)
	if err != nil {
		return nil, fmt.Errorf("failed to read message header: %w", err)
	}

	// Read payload
	msgType := protocol.MessageType(header[0])
	payloadLen := binary.BigEndian.Uint32(header[1:5])

	// Validate payload size to prevent memory exhaustion
	if payloadLen > MaxPayloadSize {
		return nil, fmt.Errorf("payload too large: %d bytes (max %d)", payloadLen, MaxPayloadSize)
	}

	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		_, err = io.ReadFull(c.conn, payload)
		if err != nil {
			return nil, fmt.Errorf("failed to read message payload: %w", err)
		}
	}

	return &protocol.Message{
		Type:    msgType,
		Payload: payload,
	}, nil
}

// SendSecureMessage sends an AES-encrypted protocol message
func (c *Client) SendSecureMessage(msg *protocol.Message) error {
	// Encrypt the payload with AES
	encryptedPayload, err := aesutil.Encrypt(msg.Payload, c.aesKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt payload: %w", err)
	}

	// Create message with encrypted payload
	encryptedMsg := protocol.NewMessage(msg.Type, encryptedPayload)
	return c.SendMessage(encryptedMsg)
}

// ReceiveSecureMessage receives and decrypts an AES-encrypted protocol message
func (c *Client) ReceiveSecureMessage() (*protocol.Message, error) {
	// Receive encrypted message
	encryptedMsg, err := c.ReceiveMessage()
	if err != nil {
		return nil, err
	}

	// Decrypt the payload
	decryptedPayload, err := aesutil.Decrypt(encryptedMsg.Payload, c.aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt payload: %w", err)
	}

	return &protocol.Message{
		Type:    encryptedMsg.Type,
		Payload: decryptedPayload,
	}, nil
}

// PerformHandshake performs RSA key exchange with the server
func (c *Client) PerformHandshake(ctx context.Context) error {
	c.logger.Info("Starting RSA handshake...")

	// Step 1: Generate AES key
	aesKey, err := aesutil.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate AES key: %w", err)
	}
	c.aesKey = aesKey
	c.logger.Info("Generated AES session key", zap.Int("key_length", len(c.aesKey)))

	// Step 2: Encrypt AES key with server's public key
	encryptedAESKey := rsautil.EncryptWithPublicKey(c.aesKey, c.serverPubKey)
	c.logger.Info("Encrypted AES key with server's public key")

	// Step 3: Send encrypted AES key to server
	handshakeMsg := protocol.NewMessage(protocol.MessageTypeHandshake, encryptedAESKey)
	if err := c.SendMessage(handshakeMsg); err != nil {
		return fmt.Errorf("failed to send encrypted AES key: %w", err)
	}

	c.logger.Info("Sent encrypted AES key to server")

	// Step 4: Wait for server's handshake confirmation
	response, err := c.ReceiveMessage()
	if err != nil {
		return fmt.Errorf("failed to receive handshake confirmation: %w", err)
	}

	if response.Type != protocol.MessageTypeResponse {
		return fmt.Errorf("unexpected message type: %v (expected response)", response.Type)
	}

	c.logger.Info("Received handshake confirmation - handshake complete")

	return nil
}

// UploadFile uploads a file to the server
func (c *Client) UploadFile(ctx context.Context, filename string) error {
	c.logger.Info("Uploading file", zap.String("filename", filename))

	// Read file
	fileData, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Create command message (file data is now included as-is, encryption happens at message level)
	// Send just the basename of the file, not the full path
	cmdPayload, err := protocol.SerializeCommand(protocol.CommandUpload, filepath.Base(filename), fileData)
	if err != nil {
		return fmt.Errorf(errSerializeCommand, err)
	}

	// Send encrypted command
	msg := protocol.NewMessage(protocol.MessageTypeCommand, cmdPayload)
	if err := c.SendSecureMessage(msg); err != nil {
		return fmt.Errorf("failed to send upload command: %w", err)
	}

	// Wait for encrypted response
	response, err := c.ReceiveSecureMessage()
	if err != nil {
		return fmt.Errorf(errReceiveResponse, err)
	}

	if response.Type != protocol.MessageTypeResponse {
		return fmt.Errorf(errUnexpectedResponse, response.Type)
	}

	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		return fmt.Errorf(errDeserializeResponse, err)
	}

	if !respMsg.Success {
		return fmt.Errorf("upload failed: %s", respMsg.Message)
	}

	c.logger.Info("File uploaded successfully", zap.String("message", respMsg.Message))
	return nil
}

// DownloadFile downloads a file from the server using chunked transfer
func (c *Client) DownloadFile(ctx context.Context, filename string, outputPath string) error {
	c.logger.Info("Downloading file", zap.String("filename", filename))

	// Create command message
	cmdPayload, err := protocol.SerializeCommand(protocol.CommandDownload, filename, nil)
	if err != nil {
		return fmt.Errorf(errSerializeCommand, err)
	}

	// Send encrypted command
	msg := protocol.NewMessage(protocol.MessageTypeCommand, cmdPayload)
	if err := c.SendSecureMessage(msg); err != nil {
		return fmt.Errorf("failed to send download command: %w", err)
	}

	// Wait for initial response
	response, err := c.ReceiveSecureMessage()
	if err != nil {
		return fmt.Errorf(errReceiveResponse, err)
	}

	if response.Type != protocol.MessageTypeResponse {
		return fmt.Errorf(errUnexpectedResponse, response.Type)
	}

	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		return fmt.Errorf(errDeserializeResponse, err)
	}

	if !respMsg.Success {
		return fmt.Errorf("download failed: %s", respMsg.Message)
	}

	c.logger.Info("Starting chunked download", zap.String("message", respMsg.Message))

	// Receive chunks and reconstruct file
	return c.receiveFileChunks(ctx, filename, outputPath)
}

// receiveFileChunks receives file chunks and reconstructs the complete file
func (c *Client) receiveFileChunks(ctx context.Context, filename string, outputPath string) error {
	var chunks []protocol.ChunkDataMessage
	var totalSize uint64
	var totalChunks uint32

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Receive all chunks
	for {
		// Wait for chunk data message
		chunkMsg, err := c.ReceiveSecureMessage()
		if err != nil {
			return fmt.Errorf("failed to receive chunk: %w", err)
		}

		// Check if this is the end of transfer (no more chunks)
		if chunkMsg.Type != protocol.MessageTypeData {
			// If we receive a response message, it might be an error or completion
			if chunkMsg.Type == protocol.MessageTypeResponse {
				respMsg, err := protocol.DeserializeResponse(chunkMsg.Payload)
				if err == nil && respMsg.Success {
					c.logger.Info("Download completed", zap.String("message", respMsg.Message))
					break
				}
			}
			return fmt.Errorf("unexpected message type during chunked download: %v", chunkMsg.Type)
		}

		// Deserialize chunk data
		chunk, err := protocol.DeserializeChunkData(chunkMsg.Payload)
		if err != nil {
			return fmt.Errorf("failed to deserialize chunk: %w", err)
		}

		// Validate chunk belongs to this file
		if chunk.Filename != filename {
			return fmt.Errorf("chunk filename mismatch: expected %s, got %s", filename, chunk.Filename)
		}

		// Store metadata from first chunk
		if len(chunks) == 0 {
			totalSize = chunk.TotalSize
			totalChunks = chunk.TotalChunks
			c.logger.Info("Receiving file chunks",
				zap.String("filename", filename),
				zap.Uint64("totalSize", totalSize),
				zap.Uint32("totalChunks", totalChunks))
		}

		// Write chunk data to file
		if _, err := file.Write(chunk.Data); err != nil {
			return fmt.Errorf("failed to write chunk %d to file: %w", chunk.ChunkIndex, err)
		}

		chunks = append(chunks, *chunk)

		// Log progress
		progress := float64(len(chunks)) / float64(totalChunks) * 100
		c.logger.Debug("Received chunk",
			zap.String("filename", filename),
			zap.Uint32("chunkIndex", chunk.ChunkIndex),
			zap.Uint32("chunkSize", chunk.ChunkSize),
			zap.Float64("progress", progress))

		// Check if we've received all chunks
		if len(chunks) >= int(totalChunks) {
			c.logger.Info("All chunks received", zap.String("filename", filename))
			break
		}
	}

	// Verify we received all chunks
	if len(chunks) != int(totalChunks) {
		return fmt.Errorf("incomplete download: received %d chunks, expected %d", len(chunks), totalChunks)
	}

	// Verify file size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if uint64(fileInfo.Size()) != totalSize {
		return fmt.Errorf("file size mismatch: expected %d bytes, got %d", totalSize, fileInfo.Size())
	}

	c.logger.Info("File downloaded successfully",
		zap.String("output", outputPath),
		zap.Uint64("size", totalSize),
		zap.Uint32("chunks", totalChunks))

	return nil
}

// ListFiles lists files on the server
func (c *Client) ListFiles(ctx context.Context) (string, error) {
	c.logger.Info("Listing files")

	// Create command message
	cmdPayload, err := protocol.SerializeCommand(protocol.CommandList, "", nil)
	if err != nil {
		return "", fmt.Errorf(errSerializeCommand, err)
	}

	// Send encrypted command
	msg := protocol.NewMessage(protocol.MessageTypeCommand, cmdPayload)
	if err := c.SendSecureMessage(msg); err != nil {
		return "", fmt.Errorf("failed to send list command: %w", err)
	}

	// Wait for encrypted response
	response, err := c.ReceiveSecureMessage()
	if err != nil {
		return "", fmt.Errorf(errReceiveResponse, err)
	}

	if response.Type != protocol.MessageTypeResponse {
		return "", fmt.Errorf(errUnexpectedResponse, response.Type)
	}

	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		return "", fmt.Errorf(errDeserializeResponse, err)
	}

	if !respMsg.Success {
		return "", fmt.Errorf("list failed: %s", respMsg.Message)
	}

	return respMsg.Message, nil
}

// DeleteFile deletes a file on the server
func (c *Client) DeleteFile(ctx context.Context, filename string) error {
	c.logger.Info("Deleting file", zap.String("filename", filename))

	// Create command message
	cmdPayload, err := protocol.SerializeCommand(protocol.CommandDelete, filename, nil)
	if err != nil {
		return fmt.Errorf(errSerializeCommand, err)
	}

	// Send encrypted command
	msg := protocol.NewMessage(protocol.MessageTypeCommand, cmdPayload)
	if err := c.SendSecureMessage(msg); err != nil {
		return fmt.Errorf("failed to send delete command: %w", err)
	}

	// Wait for encrypted response
	response, err := c.ReceiveSecureMessage()
	if err != nil {
		return fmt.Errorf(errReceiveResponse, err)
	}

	if response.Type != protocol.MessageTypeResponse {
		return fmt.Errorf(errUnexpectedResponse, response.Type)
	}

	respMsg, err := protocol.DeserializeResponse(response.Payload)
	if err != nil {
		return fmt.Errorf(errDeserializeResponse, err)
	}

	if !respMsg.Success {
		return fmt.Errorf("delete failed: %s", respMsg.Message)
	}

	c.logger.Info("File deleted successfully", zap.String("message", respMsg.Message))
	return nil
}
