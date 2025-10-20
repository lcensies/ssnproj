# Client Usage Guide

## Overview
A secure file transfer client with RSA key exchange and AES encryption over TCP.

## Building the Client

```bash
cd /Users/avdeev/Study/SSN/executables/ssnproj
go build -o bin/client ./cmd/client
```

## Running the Client

### Basic Usage
```bash
./bin/client -host localhost -port 8080
```

### Command Line Options
- `-host`: Server hostname (default: localhost)
- `-port`: Server port (default: 8080)

## Interactive Commands

Once connected, you'll see a command prompt. Available commands:

### Upload a File
```
> upload myfile.txt
> up myfile.txt          # Short alias
```
Uploads a file to the server. The file is encrypted with AES-256 before transmission.

### Download a File
```
> download myfile.txt
> download myfile.txt localcopy.txt    # Specify output filename
> dl myfile.txt                        # Short alias
```
Downloads a file from the server and decrypts it automatically.

### List Files
```
> list
> ls                     # Short alias
```
Lists all files available on the server.

### Delete a File
```
> delete myfile.txt
> rm myfile.txt          # Short alias
> del myfile.txt         # Another alias
```
Deletes a file from the server (asks for confirmation).

### Help
```
> help
> h                      # Short alias
```
Displays the help message with all available commands.

### Exit
```
> exit
> quit                   # Alternative
> q                      # Short alias
```
Disconnects from the server and exits the client.

## Security Features

### RSA Key Exchange (2048-bit)
The handshake protocol works as follows:
1. **Server sends its public RSA key** to the client (2048-bit)
2. **Client generates an AES-256 session key** 
3. **Client encrypts the AES key** with the server's public RSA key
4. **Client sends encrypted AES key** to the server
5. **Server decrypts the AES key** with its private RSA key
6. Both parties now share the same AES-256 session key

### AES-256-GCM Encryption
- All file data is encrypted with AES-256 in GCM mode
- Provides both confidentiality and authenticity
- Each encryption operation uses a unique nonce

### Protocol Structure
Messages follow a binary protocol:
- 1 byte: Message type
- 4 bytes: Payload length (big-endian)
- N bytes: Payload

Message types:
- `0x01`: Greeting
- `0x02`: Handshake (RSA key exchange)
- `0x03`: Command (file operations)
- `0x04`: Data
- `0x05`: Response

## Example Session

```
$ ./bin/client -host localhost -port 8080

╔══════════════════════════════════════════════════════════════╗
║          Secure File Transfer Client - Commands             ║
╚══════════════════════════════════════════════════════════════╝

  upload <filename>              Upload a file to the server
  download <filename> [output]   Download a file from the server
  list                           List all files on the server
  delete <filename>              Delete a file from the server
  help                           Show this help message
  exit                           Disconnect and exit

Aliases:
  up = upload  |  dl = download  |  ls = list  |  rm/del = delete

> upload test.txt
✓ File 'test.txt' uploaded successfully

> list

Files on server:
================
test.txt

> download test.txt downloaded.txt
✓ File downloaded to 'downloaded.txt'

> delete test.txt
Are you sure you want to delete 'test.txt'? (y/n): y
✓ File 'test.txt' deleted successfully

> exit
Goodbye!
```

## Troubleshooting

### Connection Refused
- Ensure the server is running
- Check the host and port are correct
- Verify firewall settings

### Handshake Failed
- Server might not be implementing the RSA handshake protocol
- Check server logs for errors

### File Not Found (Upload)
- Verify the file exists in the current directory
- Use absolute or relative path to the file

### Permission Denied
- Check file permissions
- Ensure you have read/write access to the directory

## Network Protocol: TCP vs UDP

This client uses **TCP** for the following reasons:

### Why TCP?
✅ **Reliability**: Files must arrive complete and in order
✅ **Error checking**: Automatic retransmission of lost packets
✅ **Flow control**: Prevents overwhelming slower receivers
✅ **Simplicity**: Built-in stream handling
✅ **Security**: Easier to implement encryption over streams

## Technical Details

### Dependencies
- `go.uber.org/zap`: Structured logging
- Standard Go crypto libraries

### File Locations
- Client source: `cmd/client/`
- Protocol definitions: `pkg/protocol/`
- RSA utilities: `pkg/rsa/`
- AES utilities: `pkg/aes/`

### Logging
Logs are output in JSON format (production mode) to stdout. Example:
```json
{"level":"info","ts":1729468800.123,"msg":"Connected to server","host":"localhost","port":"8080"}
```

## Contributing
When extending the client:
1. Follow the existing code structure
2. Add error handling for all operations
3. Update this documentation
4. Test with the server implementation

