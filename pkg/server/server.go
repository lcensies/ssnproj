package server

import (
	"bufio"
	"crypto/rsa"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	protocol "github.com/lcensies/ssnproj/pkg/protocol"
	rsaPkg "github.com/lcensies/ssnproj/pkg/rsa"
)

type ServerConfig struct {
	host         string
	port         string
	configFolder string
}

type Server struct {
	config     *ServerConfig
	rsaKeyPair *RSAKeyPair
}

type ConnectionState int

const (
	ConnectionStateNew ConnectionState = iota
	ConnectionStateHandshake
	ConnectionStateAuthorized
	ConnectionStateClosed
)

type RSAKeyPair struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

const defaultRsaKeySize = 2048

type ConnectionHandler struct {
	conn          net.Conn
	state         ConnectionState
	messageBuffer *protocol.MessageBuffer
	// aesKey *[]byte
}

func NewConnectionHandler(conn net.Conn) *ConnectionHandler {
	return &ConnectionHandler{
		conn:          conn,
		state:         ConnectionStateNew,
		messageBuffer: protocol.NewMessageBuffer(),
	}
}

func (handler *ConnectionHandler) handleRequest() {
	reader := bufio.NewReader(handler.conn)
	buffer := make([]byte, 1024)

	for {
		// Read data from connection
		n, err := reader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading from connection: %v\n", err)
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
				fmt.Printf("Error deserializing message: %v\n", err)
				handler.conn.Close()
				return
			}

			// Process the complete message
			fmt.Printf("Message received - Type: %d, Payload: %s\n", message.Type, string(message.Payload))

			// Send response
			response, err := protocol.NewMessage(protocol.MessageTypeResponse, nil).Serialize()
			if err != nil {
				fmt.Printf("Error serializing response: %v\n", err)
				handler.conn.Close()
				return
			}

			if _, err := handler.conn.Write(response); err != nil {
				fmt.Printf("Error writing response: %v\n", err)
				handler.conn.Close()
				return
			}

			// If there's no more data in the buffer, break out of the inner loop
			if !handler.messageBuffer.HasData() {
				break
			}
		}
	}
}

func (handler *ConnectionHandler) performHandshake() {
	handler.state = ConnectionStateHandshake
	handler.conn.Write([]byte("Handshake complete.\n"))
}

func LoadKeypair(configFolder string) (*RSAKeyPair, error) {
	if _, err := os.Stat(configFolder); os.IsNotExist(err) {
		privKey, pubKey := rsaPkg.GenerateKeyPair(defaultRsaKeySize)
		return &RSAKeyPair{
			privateKey: privKey,
			publicKey:  pubKey,
		}, nil
	}
	privKeyBytes, err := os.ReadFile(fmt.Sprintf("%s/private.pem", configFolder))
	if err != nil {
		return nil, err
	}
	pubKeyBytes, err := os.ReadFile(fmt.Sprintf("%s/public.pem", configFolder))
	if err != nil {
		return nil, err
	}
	privKey := rsaPkg.BytesToPrivateKey(privKeyBytes)
	pubKey := rsaPkg.BytesToPublicKey(pubKeyBytes)
	return &RSAKeyPair{
		privateKey: privKey,
		publicKey:  pubKey,
	}, nil
}

func NewServer(config *ServerConfig) (*Server, error) {
	rsaKeyPair, err := LoadKeypair(config.configFolder)
	if err != nil {
		return nil, err
	}
	return &Server{
		config:     config,
		rsaKeyPair: rsaKeyPair,
	}, nil
}

func (server *Server) Run() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", server.config.host, server.config.port))
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		client := NewConnectionHandler(conn)
		go client.handleRequest()
	}
}
