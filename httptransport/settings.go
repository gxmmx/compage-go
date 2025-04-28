package httptransport

import (
	"crypto"
	"crypto/x509"
)

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Settings struct {
	Host string
	Port int

	Prefix string

	SwaggerEnabled bool
	SwaggerPath    string

	SslEnabled bool
	SslCrt     *x509.Certificate
	SslKey     *crypto.PrivateKey
	SslCaCrt   *x509.Certificate
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewSettings() *Settings {
	return &Settings{
		Host:           "",
		Port:           3000,
		Prefix:         "",
		SwaggerEnabled: false,
		SwaggerPath:    "/swagger",
		SslEnabled:     false,
		SslCrt:         nil,
		SslKey:         nil,
		SslCaCrt:       nil,
	}
}
