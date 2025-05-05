package hashivault

import "crypto/x509"

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Settings struct {
	Host string
	Port int

	RoleID   string
	SecretID string

	SslCaCrt   *x509.Certificate
	SslDevMode bool
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewSettings() *Settings {
	return &Settings{
		Host:       "localhost",
		Port:       8200,
		RoleID:     "",
		SecretID:   "",
		SslCaCrt:   nil,
		SslDevMode: false,
	}
}
