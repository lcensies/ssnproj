package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

// Custom error types for message deserialization
var (
	ErrMessageNotReady   = errors.New("message not ready")
	ErrInsufficientData  = errors.New("insufficient data for message header")
	ErrIncompletePayload = errors.New("incomplete message payload")
)

// MessageType represents the type of message
type MessageType byte

const (
	// Message types
	MessageTypeHandshake MessageType = 0x01
	MessageTypeCommand   MessageType = 0x02
	MessageTypeData      MessageType = 0x03
	MessageTypeResponse  MessageType = 0x04
)

// CommandType represents different file operations
type CommandType byte

const (
	CommandUpload   CommandType = 0x01
	CommandDownload CommandType = 0x02
	CommandList     CommandType = 0x03
	CommandDelete   CommandType = 0x04
)

// Message represents a protocol message
type Message struct {
	Type    MessageType
	Payload []byte
}

// CommandMessage represents a command message
type CommandMessage struct {
	Command  CommandType
	Filename string
	Data     []byte
}

// ResponseMessage represents a response message
type ResponseMessage struct {
	Success bool
	Message string
	Data    []byte
}

// NewMessage creates a new message
func NewMessage(msgType MessageType, payload []byte) *Message {
	return &Message{
		Type:    msgType,
		Payload: payload,
	}
}

// Serialize converts a message to bytes
func (m *Message) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write message type (1 byte)
	if err := buf.WriteByte(byte(m.Type)); err != nil {
		return nil, err
	}

	// Write payload length (4 bytes)
	payloadLen := uint32(len(m.Payload))
	if err := binary.Write(buf, binary.BigEndian, payloadLen); err != nil {
		return nil, err
	}

	// Write payload
	if _, err := buf.Write(m.Payload); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Deserialize converts bytes to a message
func Deserialize(data []byte) (*Message, error) {
	if len(data) < 5 {
		return nil, errors.New("data too short")
	}

	buf := bytes.NewReader(data)

	// Read message type
	msgType, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}

	// Read payload length
	var payloadLen uint32
	if err := binary.Read(buf, binary.BigEndian, &payloadLen); err != nil {
		return nil, err
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if _, err := buf.Read(payload); err != nil {
		return nil, err
	}

	return &Message{
		Type:    MessageType(msgType),
		Payload: payload,
	}, nil
}

// MessageBuffer handles partial message reading with proper buffering
type MessageBuffer struct {
	buffer []byte
}

// NewMessageBuffer creates a new message buffer
func NewMessageBuffer() *MessageBuffer {
	return &MessageBuffer{
		buffer: make([]byte, 0),
	}
}

// AddData adds new data to the buffer
func (mb *MessageBuffer) AddData(data []byte) {
	mb.buffer = append(mb.buffer, data...)
}

// TryDeserialize attempts to deserialize a complete message from the buffer
// Returns the message and remaining buffer data if successful, or nil and error if not ready
func (mb *MessageBuffer) TryDeserialize() (*Message, error) {
	// Need at least 5 bytes (1 for type + 4 for length)
	if len(mb.buffer) < 5 {
		return nil, ErrInsufficientData
	}

	// Read payload length from the buffer
	payloadLen := binary.BigEndian.Uint32(mb.buffer[1:5])

	// Calculate total message length: 1 (type) + 4 (length) + payload
	totalMessageLen := 5 + int(payloadLen)

	// Check if we have the complete message
	if len(mb.buffer) < totalMessageLen {
		return nil, ErrIncompletePayload
	}

	// Extract the complete message
	messageData := mb.buffer[:totalMessageLen]
	remainingData := mb.buffer[totalMessageLen:]

	// Deserialize the complete message
	message, err := Deserialize(messageData)
	if err != nil {
		return nil, err
	}

	// Update buffer to contain only remaining data
	mb.buffer = remainingData

	return message, nil
}

// HasData returns true if there's data in the buffer
func (mb *MessageBuffer) HasData() bool {
	return len(mb.buffer) > 0
}

// Clear clears the buffer
func (mb *MessageBuffer) Clear() {
	mb.buffer = mb.buffer[:0]
}

// SerializeCommand serializes a command message
func SerializeCommand(cmd CommandType, filename string, data []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write command type (1 byte)
	if err := buf.WriteByte(byte(cmd)); err != nil {
		return nil, err
	}

	// Write filename length (2 bytes)
	filenameLen := uint16(len(filename))
	if err := binary.Write(buf, binary.BigEndian, filenameLen); err != nil {
		return nil, err
	}

	// Write filename
	if _, err := buf.WriteString(filename); err != nil {
		return nil, err
	}

	// Write data
	if _, err := buf.Write(data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DeserializeCommand deserializes a command message
func DeserializeCommand(data []byte) (*CommandMessage, error) {
	if len(data) < 3 {
		return nil, errors.New("command data too short")
	}

	buf := bytes.NewReader(data)

	// Read command type
	cmdType, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}

	// Read filename length
	var filenameLen uint16
	if err := binary.Read(buf, binary.BigEndian, &filenameLen); err != nil {
		return nil, err
	}

	// Read filename
	filename := make([]byte, filenameLen)
	if _, err := buf.Read(filename); err != nil {
		return nil, err
	}

	// Read remaining data
	remaining := make([]byte, buf.Len())
	if _, err := buf.Read(remaining); err != nil && err != io.EOF {
		return nil, err
	}

	return &CommandMessage{
		Command:  CommandType(cmdType),
		Filename: string(filename),
		Data:     remaining,
	}, nil
}

// SerializeResponse serializes a response message
func SerializeResponse(success bool, message string, data []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write success flag (1 byte)
	successByte := byte(0)
	if success {
		successByte = 1
	}
	if err := buf.WriteByte(successByte); err != nil {
		return nil, err
	}

	// Write message length (2 bytes)
	messageLen := uint16(len(message))
	if err := binary.Write(buf, binary.BigEndian, messageLen); err != nil {
		return nil, err
	}

	// Write message
	if _, err := buf.WriteString(message); err != nil {
		return nil, err
	}

	// Write data
	if _, err := buf.Write(data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DeserializeResponse deserializes a response message
func DeserializeResponse(data []byte) (*ResponseMessage, error) {
	if len(data) < 3 {
		return nil, errors.New("response data too short")
	}

	buf := bytes.NewReader(data)

	// Read success flag
	successByte, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	success := successByte == 1

	// Read message length
	var messageLen uint16
	if err := binary.Read(buf, binary.BigEndian, &messageLen); err != nil {
		return nil, err
	}

	// Read message
	message := make([]byte, messageLen)
	if _, err := buf.Read(message); err != nil {
		return nil, err
	}

	// Read remaining data
	remaining := make([]byte, buf.Len())
	if _, err := buf.Read(remaining); err != nil && err != io.EOF {
		return nil, err
	}

	return &ResponseMessage{
		Success: success,
		Message: string(message),
		Data:    remaining,
	}, nil
}
