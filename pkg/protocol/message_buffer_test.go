package protocol

import (
	"testing"
)

func TestMessageBuffer_PartialMessage(t *testing.T) {
	// Create a test message
	originalMessage := NewMessage(MessageTypeCommand, []byte("test payload"))
	serialized, err := originalMessage.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}

	// Create message buffer
	buffer := NewMessageBuffer()

	// Test with partial data (just the header)
	header := serialized[:5] // Only type + length
	buffer.AddData(header)

	// Try to deserialize - should get "not ready" error
	message, err := buffer.TryDeserialize()
	if err == nil {
		t.Error("Expected error for incomplete message, got nil")
	}
	if err != ErrIncompletePayload {
		t.Errorf("Expected ErrIncompletePayload, got %v", err)
	}
	if message != nil {
		t.Error("Expected nil message for incomplete data")
	}

	// Add the rest of the message
	payload := serialized[5:]
	buffer.AddData(payload)

	// Now should be able to deserialize
	message, err = buffer.TryDeserialize()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if message == nil {
		t.Error("Expected message, got nil")
	}
	if message.Type != MessageTypeCommand {
		t.Errorf("Expected message type %d, got %d", MessageTypeCommand, message.Type)
	}
	if string(message.Payload) != "test payload" {
		t.Errorf("Expected payload 'test payload', got '%s'", string(message.Payload))
	}
}

func TestMessageBuffer_MultipleMessages(t *testing.T) {
	// Create two test messages
	msg1 := NewMessage(MessageTypeCommand, []byte("first"))
	msg2 := NewMessage(MessageTypeResponse, []byte("second"))

	serialized1, err := msg1.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize message 1: %v", err)
	}
	serialized2, err := msg2.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize message 2: %v", err)
	}

	// Combine both messages
	combined := append(serialized1, serialized2...)

	// Create message buffer and add all data at once
	buffer := NewMessageBuffer()
	buffer.AddData(combined)

	// Deserialize first message
	message1, err := buffer.TryDeserialize()
	if err != nil {
		t.Errorf("Unexpected error deserializing first message: %v", err)
	}
	if message1.Type != MessageTypeCommand {
		t.Errorf("Expected first message type %d, got %d", MessageTypeCommand, message1.Type)
	}
	if string(message1.Payload) != "first" {
		t.Errorf("Expected first message payload 'first', got '%s'", string(message1.Payload))
	}

	// Deserialize second message
	message2, err := buffer.TryDeserialize()
	if err != nil {
		t.Errorf("Unexpected error deserializing second message: %v", err)
	}
	if message2.Type != MessageTypeResponse {
		t.Errorf("Expected second message type %d, got %d", MessageTypeResponse, message2.Type)
	}
	if string(message2.Payload) != "second" {
		t.Errorf("Expected second message payload 'second', got '%s'", string(message2.Payload))
	}
}

func TestMessageBuffer_InsufficientHeader(t *testing.T) {
	buffer := NewMessageBuffer()

	// Add only 3 bytes (less than the 5 needed for header)
	buffer.AddData([]byte{0x01, 0x00, 0x00})

	message, err := buffer.TryDeserialize()
	if err == nil {
		t.Error("Expected error for insufficient header data, got nil")
	}
	if err != ErrInsufficientData {
		t.Errorf("Expected ErrInsufficientData, got %v", err)
	}
	if message != nil {
		t.Error("Expected nil message for insufficient data")
	}
}

func TestMessageBuffer_EmptyBuffer(t *testing.T) {
	buffer := NewMessageBuffer()

	message, err := buffer.TryDeserialize()
	if err == nil {
		t.Error("Expected error for empty buffer, got nil")
	}
	if err != ErrInsufficientData {
		t.Errorf("Expected ErrInsufficientData, got %v", err)
	}
	if message != nil {
		t.Error("Expected nil message for empty buffer")
	}
}

func TestMessageBuffer_HasData(t *testing.T) {
	buffer := NewMessageBuffer()

	// Initially should have no data
	if buffer.HasData() {
		t.Error("Expected no data initially")
	}

	// Add some data
	buffer.AddData([]byte{0x01, 0x00, 0x00})
	if !buffer.HasData() {
		t.Error("Expected data after adding")
	}

	// Clear buffer
	buffer.Clear()
	if buffer.HasData() {
		t.Error("Expected no data after clearing")
	}
}

func TestMessageBuffer_LargeMessage(t *testing.T) {
	// Create a large payload
	largePayload := make([]byte, 10000)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	originalMessage := NewMessage(MessageTypeData, largePayload)
	serialized, err := originalMessage.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize large message: %v", err)
	}

	buffer := NewMessageBuffer()

	// Add data in chunks to simulate network reading
	chunkSize := 1000
	for i := 0; i < len(serialized); i += chunkSize {
		end := i + chunkSize
		if end > len(serialized) {
			end = len(serialized)
		}
		buffer.AddData(serialized[i:end])
	}

	// Should be able to deserialize the complete message
	message, err := buffer.TryDeserialize()
	if err != nil {
		t.Errorf("Unexpected error deserializing large message: %v", err)
	}
	if message == nil {
		t.Error("Expected message, got nil")
	}
	if message.Type != MessageTypeData {
		t.Errorf("Expected message type %d, got %d", MessageTypeData, message.Type)
	}
	if len(message.Payload) != len(largePayload) {
		t.Errorf("Expected payload length %d, got %d", len(largePayload), len(message.Payload))
	}
}
