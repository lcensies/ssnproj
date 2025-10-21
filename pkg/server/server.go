package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"

	aesUtil "github.com/lcensies/ssnproj/pkg/aes"
	protocol "github.com/lcensies/ssnproj/pkg/protocol"
	rsaUtil "github.com/lcensies/ssnproj/pkg/rsa"
	"go.uber.org/zap"
)

type ServerConfig struct {
	Host         string
	Port         string
	ConfigFolder string
	RootDir      *string
}

const defaultRootDir = "data"

type Server struct {
	config     *ServerConfig
	rsaKeyPair *rsaUtil.RSAKeyPair
	logger     *zap.Logger
}

type ConnectionState int

const (
	ConnectionStateNew ConnectionState = iota
	ConnectionStateHandshake
	ConnectionStateAuthenticated
	ConnectionStateClosed
)

type ConnectionHandler struct {
	conn          net.Conn
	state         ConnectionState
	messageBuffer *protocol.MessageBuffer
	aesKey        []byte
	rsaKeyPair    *rsaUtil.RSAKeyPair
	logger        *zap.Logger
	cmdHandler    *CommandHandler
}

func (c *ConnectionHandler) SendSecureMessage(message *protocol.Message) error {
	// Encrypt the payload with AES
	encryptedPayload, err := aesUtil.Encrypt(message.Payload, c.aesKey)
	if err != nil {
		return err
	}

	// Create message with encrypted payload
	encryptedMsg := protocol.NewMessage(message.Type, encryptedPayload)
	serializedMsg, err := encryptedMsg.Serialize()

	if err != nil {
		return err
	}
	_, err = c.conn.Write(serializedMsg)
	if err != nil {
		return err
	}
	return nil
}

func NewConnectionHandler(
	conn net.Conn,
	rsaKeyPair *rsaUtil.RSAKeyPair,
	logger *zap.Logger,
	rootDir *string) *ConnectionHandler {

	handler := &ConnectionHandler{
		conn:          conn,
		state:         ConnectionStateNew,
		messageBuffer: protocol.NewMessageBuffer(),
		rsaKeyPair:    rsaKeyPair,
		logger:        logger,
		cmdHandler:    nil,
	}
	handler.cmdHandler = NewCommandHandler(handler, logger, rootDir)
	return handler
}

func (handler *ConnectionHandler) handleHandshake(m *protocol.Message) error {
	handler.state = ConnectionStateHandshake

	// Decrypt the AES key sent by the client
	aesKey := rsaUtil.DecryptWithPrivateKey(m.Payload, handler.rsaKeyPair.Private)
	handler.aesKey = aesKey

	// Send confirmation response
	response, err := protocol.NewMessage(protocol.MessageTypeResponse, []byte("handshake complete")).Serialize()
	if err != nil {
		return fmt.Errorf("error serializing handshake response: %v", err)
	}
	_, err = handler.conn.Write(response)
	if err != nil {
		return fmt.Errorf("error sending handshake response: %v", err)
	}

	handler.state = ConnectionStateAuthenticated
	return nil
}

func (handler *ConnectionHandler) handleCommand(message *protocol.Message) error {
	command, err := protocol.DeserializeCommand(message.Payload)
	if err != nil {
		return err
	}

	return handler.cmdHandler.handle(command)
}

func (handler *ConnectionHandler) handleMessage(message *protocol.Message) error {
	if message.Type == protocol.MessageTypeHandshake {
		return handler.handleHandshake(message)
	}

	// Only decrypt if we have an AES key (after handshake)
	if handler.aesKey == nil {
		return fmt.Errorf("received non-handshake message before handshake complete")
	}

	err := message.Decrypt(handler.aesKey)
	if err != nil {
		return err
	}

	switch message.Type {
	case protocol.MessageTypeCommand:
		return handler.handleCommand(message)
	default:
		return fmt.Errorf("unexpected message type: %v", message.Type)
	}
}

func (handler *ConnectionHandler) HandleRawRequest() {
	reader := bufio.NewReader(handler.conn)
	buffer := make([]byte, 1024)

	for {
		// Read data from connection
		n, err := reader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				handler.logger.Error("Error reading from connection", zap.Error(err))
			}
			handler.conn.Close()
			return
		}

		// Add received data to message buffer
		handler.messageBuffer.AddData(buffer[:n])

		// Try to deserialize complete messages from the buffer
		for {
			message, err := handler.messageBuffer.TryDeserialize()
			if err != nil {
				// Check if it's a "not ready" error - this is expected for partial messages
				if err == protocol.ErrInsufficientData || err == protocol.ErrIncompletePayload {
					// Message not complete yet, wait for more data
					break
				}
				// Other errors are actual problems
				handler.logger.Error("Error deserializing message", zap.Error(err))
				handler.conn.Close()
				return
			}

			// Process the complete message
			err = handler.handleMessage(message)
			if err != nil {
				handler.logger.Error("Error handling message", zap.Error(err))
				handler.conn.Close()
				return
			}

		}
	}
}

func NewServer(config *ServerConfig) (*Server, error) {
	// TODO: Use a proper logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	rsaKeyPair, err := rsaUtil.LoadKeypair(config.ConfigFolder)
	if err != nil {
		return nil, err
	}
	return &Server{
		config:     config,
		rsaKeyPair: rsaKeyPair,
		logger:     logger,
	}, nil
}

// SetRSAKeyPair sets the RSA key pair for testing purposes
func (server *Server) SetRSAKeyPair(keyPair *rsaUtil.RSAKeyPair) {
	server.rsaKeyPair = keyPair
}

func (server *Server) Run() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", server.config.Host, server.config.Port))
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		client := NewConnectionHandler(conn, server.rsaKeyPair, server.logger, server.config.RootDir)
		go client.HandleRawRequest()
	}
}
