package util

import (
	"io"
	"net"
	"dialtone/cli/src/core/logger"
)

// ProxyListener accepts connections and proxies them to the target address
func ProxyListener(ln net.Listener, targetAddr string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Listener closed
			return
		}
		go ProxyConnection(conn, targetAddr)
	}
}

// ProxyConnection proxies data between source and destination
func ProxyConnection(src net.Conn, targetAddr string) {
	defer src.Close()

	// Apply TCP optimizations if possible
	if tcpSrc, ok := src.(*net.TCPConn); ok {
		tcpSrc.SetNoDelay(true)
		tcpSrc.SetReadBuffer(64 * 1024)
		tcpSrc.SetWriteBuffer(64 * 1024)
	}

	dst, err := net.Dial("tcp", targetAddr)
	if err != nil {
		logger.LogInfo("Failed to connect to proxy backend: %v", err)
		return
	}
	defer dst.Close()

	if tcpDst, ok := dst.(*net.TCPConn); ok {
		tcpDst.SetNoDelay(true)
		tcpDst.SetReadBuffer(64 * 1024)
		tcpDst.SetWriteBuffer(64 * 1024)
	}

	// Bidirectional copy
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(dst, src)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(src, dst)
		done <- struct{}{}
	}()

	// Wait for either direction to complete
	<-done
}
