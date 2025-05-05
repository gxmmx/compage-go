package postgres

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
)

func getHostCaCertificate(host string, port int) (*x509.Certificate, error) {
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed make tcp connection to host: %w", err)
	}
	defer conn.Close()

	// PostgreSQL SSLRequest packet
	sslRequest := []byte{
		0x00, 0x00, 0x00, 0x08, // length = 8
		0x04, 0xD2, 0x16, 0x2F, // magic number = 80877103
	}
	if _, err := conn.Write(sslRequest); err != nil {
		return nil, fmt.Errorf("failed to send ssl request: %w", err)
	}

	resp := make([]byte, 1)
	if _, err := conn.Read(resp); err != nil {
		return nil, fmt.Errorf("failed to read ssl response: %w", err)
	}

	if resp[0] != 'S' {
		// Server doesn't support TLS
		fmt.Println("Server doesn't support TLS")
		return nil, nil
	}

	// Upgrade to TLS
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true, // We only want the certs, no validation
		ServerName:         host,
	})
	if err := tlsConn.Handshake(); err != nil {
		return nil, fmt.Errorf("failed to perform tls handshake: %w", err)
	}
	defer tlsConn.Close()

	// Get peer certificates
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		// TLS was used, but no certs presented — rare but possible
		return nil, nil
	}
	// Return the first (CA) certificate
	return certs[0], nil
}
