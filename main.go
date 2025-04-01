package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	bufferSize = 128 * 1024 // 128KB buffer for higher throughput
)

type Forwarder struct {
	sourceAddr string
	targetAddr string
	isUnix     bool
}

func NewForwarder(source, target string) *Forwarder {
	isUnix := false
	if _, err := os.Stat(source); err == nil {
		isUnix = true
	}
	return &Forwarder{
		sourceAddr: source,
		targetAddr: target,
		isUnix:     isUnix,
	}
}

func optimizeConn(conn net.Conn) error {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// Disable Nagle's algorithm
		if err := tcpConn.SetNoDelay(true); err != nil {
			return fmt.Errorf("failed to set TCP_NODELAY: %v", err)
		}
		// Set TCP keepalive
		if err := tcpConn.SetKeepAlive(true); err != nil {
			return fmt.Errorf("failed to set TCP keepalive: %v", err)
		}
		// Set keepalive period to 30 seconds
		if err := tcpConn.SetKeepAlivePeriod(30 * time.Second); err != nil {
			return fmt.Errorf("failed to set TCP keepalive period: %v", err)
		}

		// Get the underlying file descriptor
		file, err := tcpConn.File()
		if err != nil {
			return fmt.Errorf("failed to get file descriptor: %v", err)
		}
		defer file.Close()

		// Set socket options for high throughput
		if err := syscall.SetsockoptInt(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_RCVBUF, 1024*1024); err != nil {
			return fmt.Errorf("failed to set SO_RCVBUF: %v", err)
		}
		if err := syscall.SetsockoptInt(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1024*1024); err != nil {
			return fmt.Errorf("failed to set SO_SNDBUF: %v", err)
		}
	}
	return nil
}

// OptimizedWriter implements a zero-copy writer
type OptimizedWriter struct {
	conn net.Conn
}

func (w *OptimizedWriter) Write(p []byte) (n int, err error) {
	return w.conn.Write(p)
}

func (f *Forwarder) Start() error {
	var listener net.Listener
	var err error

	if f.isUnix {
		listener, err = net.Listen("unix", f.sourceAddr)
	} else {
		listener, err = net.Listen("tcp", f.sourceAddr)
	}

	if err != nil {
		return fmt.Errorf("failed to start listener: %v", err)
	}
	defer listener.Close()

	log.Printf("Forwarding from %s to %s\n", f.sourceAddr, f.targetAddr)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		listener.Close()
		os.Exit(0)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v\n", err)
			continue
		}

		if err := optimizeConn(conn); err != nil {
			log.Printf("Failed to optimize connection: %v\n", err)
			conn.Close()
			continue
		}

		go f.handleConnection(conn)
	}
}

func (f *Forwarder) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	var targetConn net.Conn
	var err error

	if f.isUnix {
		targetConn, err = net.Dial("unix", f.targetAddr)
	} else {
		targetConn, err = net.Dial("tcp", f.targetAddr)
	}

	if err != nil {
		log.Printf("Failed to connect to target: %v\n", err)
		return
	}
	defer targetConn.Close()

	if err := optimizeConn(targetConn); err != nil {
		log.Printf("Failed to optimize target connection: %v\n", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Create optimized writers for zero-copy
	clientWriter := &OptimizedWriter{conn: clientConn}
	targetWriter := &OptimizedWriter{conn: targetConn}

	// Use io.Copy with optimized writers for zero-copy transfer
	go func() {
		defer wg.Done()
		io.Copy(targetWriter, clientConn)
	}()

	go func() {
		defer wg.Done()
		io.Copy(clientWriter, targetConn)
	}()

	wg.Wait()
}

func main() {
	source := flag.String("source", "", "Source address (Unix socket path or TCP port)")
	target := flag.String("target", "", "Target address (Unix socket path or TCP port)")
	flag.Parse()

	if *source == "" || *target == "" {
		log.Fatal("Both source and target addresses must be specified")
	}

	forwarder := NewForwarder(*source, *target)
	if err := forwarder.Start(); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
