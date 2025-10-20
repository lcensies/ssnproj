# Secure File Transfer Protocol Specification

## Overview
A custom binary protocol for secure file transfer over TCP using RSA key exchange and AES-256-GCM encryption.

## Connection Flow

```
Client                                Server
  |                                     |
  |---- TCP Connection Established --->|
  |                                     |
  |<------- Server Public Key ---------|  (RSA-2048, MessageTypeHandshake)
  |                                     |
  | Generate AES-256 key                |
  | Encrypt with Server's Public Key    |
  |                                     |
  |------- Encrypted AES Key --------->|  (MessageTypeHandshake)
  |                                     |  Server decrypts AES key
  |                                     |
  | Both parties now share AES key      |
  |                                     |
  |------- Command (encrypted) ------->|  (MessageTypeCommand)
  |                                     |
  |<------ Response (encrypted) -------|  (MessageTypeResponse)
  |                                     |
  |------- exit command -------------->|
  |                                     |
  |---- TCP Connection Closed -------->|
```

## Message Format

All messages follow this binary structure:

```
+----------+----------------+-----------------+
| Type     | Payload Length | Payload         |
| (1 byte) | (4 bytes)      | (N bytes)       |
+----------+----------------+-----------------+
```

### Fields

1. **Type** (1 byte): Message type identifier
2. **Payload Length** (4 bytes, big-endian): Length of payload in bytes
3. **Payload** (N bytes): Message-specific data

## Message Types

| Type | Value | Description |
|------|-------|-------------|
| MessageTypeHandshake | 0x01 | RSA key exchange |
| MessageTypeCommand | 0x02 | File operation command |
| MessageTypeData | 0x03 | Raw data transfer (reserved) |
| MessageTypeResponse | 0x04 | Server response |

## Handshake Protocol

### Step 1: Server Sends Public Key

**Direction:** Server → Client  
**Message Type:** `MessageTypeHandshake` (0x01)  
**Payload:** PEM-encoded RSA public key (2048-bit)

```
Server Public Key (PEM format):
-----BEGIN RSA PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END RSA PUBLIC KEY-----
```

### Step 2: Client Sends Encrypted AES Key

**Direction:** Client → Server  
**Message Type:** `MessageTypeHandshake` (0x01)  
**Payload:** AES-256 key encrypted with server's public RSA key (OAEP-SHA512)

- Client generates a random 32-byte (256-bit) AES key
- Encrypts it using RSA-OAEP with SHA-512
- Sends encrypted key to server
- Server decrypts using its private RSA key

## Command Protocol

### Command Message Structure

After handshake, all commands are sent as `MessageTypeCommand` with this payload structure:

```
+-------------+----------------+-------------+-----------+
| Command     | Filename Len   | Filename    | Data      |
| (1 byte)    | (2 bytes)      | (N bytes)   | (M bytes) |
+-------------+----------------+-------------+-----------+
```

### Command Types

| Command | Value | Description |
|---------|-------|-------------|
| CommandUpload | 0x01 | Upload file to server |
| CommandDownload | 0x02 | Download file from server |
| CommandList | 0x03 | List files on server |
| CommandDelete | 0x04 | Delete file from server |

### Command Details

#### Upload Command (0x01)

**Payload:**
- Command: `0x01`
- Filename Length: 2 bytes (big-endian)
- Filename: UTF-8 string
- Data: **AES-256-GCM encrypted file contents**

The file data is encrypted using AES-256-GCM with the shared session key.

#### Download Command (0x02)

**Payload:**
- Command: `0x02`
- Filename Length: 2 bytes (big-endian)
- Filename: UTF-8 string
- Data: (empty)

#### List Command (0x03)

**Payload:**
- Command: `0x03`
- Filename Length: `0x0000`
- Filename: (empty)
- Data: (empty)

#### Delete Command (0x04)

**Payload:**
- Command: `0x04`
- Filename Length: 2 bytes (big-endian)
- Filename: UTF-8 string
- Data: (empty)

## Response Protocol

### Response Message Structure

Server responses use `MessageTypeResponse` with this payload:

```
+---------+---------------+----------+-----------+
| Success | Message Len   | Message  | Data      |
| (1 byte)| (2 bytes)     | (N bytes)| (M bytes) |
+---------+---------------+----------+-----------+
```

### Fields

1. **Success** (1 byte): `0x01` = success, `0x00` = failure
2. **Message Length** (2 bytes, big-endian): Length of status message
3. **Message** (N bytes): Human-readable status message (UTF-8)
4. **Data** (M bytes): Response data (if applicable)

### Response Data

- **Upload**: Data field is empty
- **Download**: Data field contains **AES-256-GCM encrypted file contents**
- **List**: Data field is empty (file list is in Message field)
- **Delete**: Data field is empty

## Encryption

### RSA-OAEP (Key Exchange)

- **Algorithm:** RSA with OAEP padding
- **Key Size:** 2048 bits
- **Hash Function:** SHA-512
- **Usage:** Encrypt AES session key only

### AES-256-GCM (Data Encryption)

- **Algorithm:** AES in Galois/Counter Mode
- **Key Size:** 256 bits (32 bytes)
- **Nonce Size:** 12 bytes (automatically generated per encryption)
- **Tag Size:** 128 bits (authentication tag)
- **Usage:** Encrypt all file data

#### AES-GCM Message Format

```
+---------------+-------------------+----------------+
| Nonce         | Encrypted Data    | Auth Tag       |
| (12 bytes)    | (N bytes)         | (16 bytes)     |
+---------------+-------------------+----------------+
```

The nonce is prepended to the ciphertext and used for decryption.

## Security Considerations

### Confidentiality
- All file data is encrypted with AES-256-GCM
- Session key is unique per connection
- RSA-2048 protects the key exchange

### Integrity
- AES-GCM provides authenticated encryption
- Any tampering with ciphertext will be detected during decryption

### Authentication
- Currently: No peer authentication (vulnerable to MITM)
- **Recommendation:** Add certificate-based authentication or pre-shared keys

### Forward Secrecy
- ❌ **Not Implemented:** Same server RSA key used for all connections
- **Recommendation:** Implement ephemeral Diffie-Hellman (DHE) for perfect forward secrecy

### Replay Protection
- ❌ **Not Implemented:** No sequence numbers or timestamps
- **Recommendation:** Add message sequence numbers

## Error Handling

### Connection Errors
- TCP connection failures: Client logs error and exits
- Timeout: Default TCP timeout applies

### Protocol Errors
- Invalid message type: Connection closed
- Payload size mismatch: Message rejected
- Deserialization failure: Connection closed

### Cryptographic Errors
- Decryption failure: Operation rejected, connection remains open
- Invalid key: Handshake failure, connection closed

## Example Message Exchanges

### Successful Upload

```
1. Client → Server: MessageTypeCommand
   - Command: 0x01 (Upload)
   - Filename: "document.txt"
   - Data: <AES-encrypted file contents>

2. Server → Client: MessageTypeResponse
   - Success: 0x01
   - Message: "File uploaded successfully"
   - Data: (empty)
```

### Failed Download

```
1. Client → Server: MessageTypeCommand
   - Command: 0x02 (Download)
   - Filename: "nonexistent.txt"
   - Data: (empty)

2. Server → Client: MessageTypeResponse
   - Success: 0x00
   - Message: "File not found"
   - Data: (empty)
```

### List Files

```
1. Client → Server: MessageTypeCommand
   - Command: 0x03 (List)
   - Filename: ""
   - Data: (empty)

2. Server → Client: MessageTypeResponse
   - Success: 0x01
   - Message: "file1.txt\nfile2.pdf\nimage.jpg"
   - Data: (empty)
```

## Implementation Notes

### Server Requirements
1. Listen for TCP connections on specified port
2. Send RSA public key immediately after connection
3. Receive and decrypt client's AES key
4. Process commands and send responses
5. Maintain file storage directory

### Client Requirements
1. Connect via TCP to server
2. Receive server's public key
3. Generate AES session key
4. Encrypt and send AES key
5. Provide interactive CLI for operations

### Performance Considerations
- RSA operations (key exchange only): ~1-10ms
- AES encryption: ~100-500 MB/s (depending on hardware)
- TCP overhead: Minimal for file transfer
- Recommended max file size: Limited by available memory (loads entire file)

## Future Enhancements

1. **Chunked Transfer:** Support streaming large files in chunks
2. **Compression:** Add optional GZIP compression before encryption
3. **Resume Support:** Allow interrupted transfers to resume
4. **Multiple Files:** Batch upload/download operations
5. **Authentication:** Add user authentication system
6. **TLS Integration:** Consider using TLS instead of custom crypto
7. **DHE Key Exchange:** Implement Diffie-Hellman for forward secrecy

