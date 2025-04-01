# GoPortForward

A high-performance port forwarding tool written in Go that supports both TCP ports and Unix domain sockets.

## Installation

```bash
git clone https://github.com/maikirakiwi/goportforward.git
cd goportforward
go build -o goportforward
```

## Usage

```bash
./goportforward -source <source> -target <target>
```

### Parameters

- `-source`: Source address (Unix socket path or port)
- `-target`: Target address (Unix socket path or port)

### Examples

1. TCP Port to TCP Port:
```bash
./goportforward -source ":8080" -target "localhost:9090"
```

2. Unix Socket to Unix Socket:
```bash
./goportforward -source "/tmp/source.sock" -target "/tmp/target.sock"
```

3. Unix Socket to TCP Port:
```bash
./goportforward -source "/tmp/source.sock" -target "localhost:8080"
```

## Requirements

- Go 1.23.5 or later
- Unix-like operating system (Linux, macOS, etc.)

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

