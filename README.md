# File Transfer System with RSA and AES

A secure file transfer system built in Go using RSA for key exchange and AES for data encryption.

## Project Structure

```
filetransfer-go/
├── cmd/                    # Main applications
│   ├── client/            # Client application entry point
│   └── server/            # Server application entry point
├── internal/              # Private application code
│   ├── config/           # Configuration management
│   ├── crypto/           # Cryptographic operations
│   └── transfer/         # File transfer logic
├── pkg/                   # Public library code
│   ├── aes/              # AES encryption utilities
│   ├── rsa/              # RSA encryption utilities
│   └── utils/            # General utilities
├── configs/               # Configuration files
└── docs/                  # Documentation
```

## Directory Descriptions

### `cmd/`
Contains the main applications:
- **`client/`**: Client application that connects to the server and transfers files
- **`server/`**: Server application that accepts connections and handles file transfers

### `internal/`
Private application-specific code:
- **`config/`**: Configuration management and settings
- **`crypto/`**: Cryptographic operations and key management
- **`transfer/`**: File transfer protocol implementation and logic

### `pkg/`
Reusable library code that can be imported by other projects:
- **`aes/`**: AES encryption/decryption utilities
- **`rsa/`**: RSA key generation, encryption, and decryption utilities
- **`utils/`**: General utility functions and helpers

### `configs/`
Configuration files for different environments and settings.

### `docs/`
Project documentation, API references, and guides.

## Security Features

- **RSA**: Used for secure key exchange between client and server
- **AES**: Used for encrypting the actual file data during transfer
- **Hybrid Encryption**: Combines the benefits of both asymmetric (RSA) and symmetric (AES) encryption

## Getting Started

### Prerequisites

- Go 1.21 or later
- Linux/macOS/Windows

### Building the Project

```bash
# Clone the repository
git clone <repository-url>
cd ssnproj

# Build the server
go build -o bin/server cmd/server/main.go

# Build the client
go build -o bin/client cmd/client/main.go
```

## Usage

### Server

The server provides a secure file transfer service with automatic configuration generation.

#### Basic Usage

```bash
# Run with default settings (localhost:8080)
./bin/server

# Run with custom port
./bin/server -port 9000

# Run with custom host and port
./bin/server -host 0.0.0.0 -port 9000
```

#### Configuration Options

The server supports both command-line flags and environment variables:

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `-host` | `SERVER_HOST` | `localhost` | Server host address |
| `-port` | `SERVER_PORT` | `8080` | Server port |
| `-config` | `SERVER_CONFIG_FOLDER` | `configs/server` | Configuration folder path |
| `-root-dir` | `SERVER_ROOT_DIR` | `data` | Root directory for file operations |
| `-log-level` | `SERVER_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `-help` | - | - | Show help message |

#### Examples

```bash
# Run with environment variables
SERVER_PORT=9000 SERVER_LOG_LEVEL=debug ./bin/server

# Run with custom config and data directories
./bin/server -config /etc/ssnproj -root-dir /var/lib/ssnproj

# Run in production mode
SERVER_HOST=0.0.0.0 SERVER_PORT=443 SERVER_LOG_LEVEL=warn ./bin/server

# Show help
./bin/server --help
```

#### Automatic Configuration

The server automatically:
- **Generates RSA key pairs** if they don't exist in the config folder
- **Creates directories** (config and data) if they don't exist
- **Sets proper permissions** (private key: 600, public key: 644, directories: 755)

### Client

The client connects to the server and performs secure file operations.

#### Basic Usage

```bash
# Connect to server and perform operations
./bin/client <server-host> <server-port>
```

#### Available Commands

- **Upload**: Upload a file to the server
- **Download**: Download a file from the server
- **List**: List files on the server
- **Delete**: Delete a file from the server

#### Examples

```bash
# Connect to local server
./bin/client localhost 8080

# Connect to remote server
./bin/client example.com 9000
```

### Protocol

The system uses a custom Secure File Transfer Protocol (SFTP) with the following features:

- **RSA-OAEP** for secure key exchange (2048-bit keys, SHA-512 hash)
- **AES-256-GCM** for data encryption (256-bit key, 12-byte nonce, 16-byte tag)
- **Chunked file transfer** for large files (64KB chunks with progress tracking)
- **Binary protocol** over TCP for efficient communication

#### Message Types

- `MessageTypeHandshake` (0x01): RSA key exchange
- `MessageTypeCommand` (0x02): Client commands (upload, download, list, delete)
- `MessageTypeData` (0x03): File data (chunked for large files)
- `MessageTypeResponse` (0x04): Server responses

#### Commands

- `CommandUpload` (0x01): Upload a file
- `CommandDownload` (0x02): Download a file
- `CommandList` (0x03): List files
- `CommandDelete` (0x04): Delete a file

For detailed protocol information, see [PROTOCOL.md](PROTOCOL.md).

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific test packages
go test ./pkg/server -v
go test ./pkg/client -v
go test ./pkg/rsa -v
go test ./pkg/aes -v

# Run E2E tests
go test ./pkg/server -v -run "TestRealE2E"
```

### Project Structure

```
ssnproj/
├── cmd/                    # Main applications
│   ├── client/            # Client application
│   └── server/            # Server application
├── pkg/                   # Public library code
│   ├── aes/              # AES encryption utilities
│   ├── client/           # Client implementation
│   ├── protocol/         # Protocol definitions
│   ├── rsa/              # RSA encryption utilities
│   └── server/           # Server implementation
├── configs/               # Configuration files
│   └── server/           # Server configuration
├── data/                  # Server data directory
├── docs/                  # Documentation
├── internal/              # Private application code
└── bin/                   # Built binaries
```

## Security Considerations

- **RSA Public Key**: The server's public key can be safely distributed publicly
- **Key Exchange**: Uses RSA-OAEP with SHA-512 for secure key exchange
- **Data Encryption**: Uses AES-256-GCM for authenticated encryption
- **Chunked Transfer**: Large files are transferred in 64KB chunks for better performance
- **Automatic Key Generation**: RSA keys are generated automatically on first run

## Troubleshooting

### Common Issues

1. **Port already in use**: Change the port using `-port` flag or `SERVER_PORT` environment variable
2. **Permission denied**: Ensure the server has write permissions to the config and data directories
3. **Connection refused**: Check if the server is running and the host/port are correct

### Logs

The server provides detailed logging at different levels:
- **debug**: Detailed information for debugging
- **info**: General information about server operations
- **warn**: Warning messages
- **error**: Error messages only

### Configuration Files

- **Private Key**: `configs/server/private.pem` (600 permissions)
- **Public Key**: `configs/server/public.pem` (644 permissions)
- **Data Directory**: `data/` (755 permissions)

## License

*License information will be added...*
