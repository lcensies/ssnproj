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

*Instructions will be added as the project develops...*

## License

*License information will be added...*
