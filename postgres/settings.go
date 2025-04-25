package postgres

import (
	"crypto/x509"
	"time"
)

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Settings struct {
	Host string
	Port int
	Name string
	User string
	Pass string

	SslMode  string
	SslCaCrt *x509.Certificate
	// Trust certificate authority from host
	SslDevMode bool

	MaxConns          int
	MinConns          int
	MaxConnLifetime   time.Duration
	HealthCheckPeriod time.Duration
	QueryTimeout      time.Duration
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewSettings() *Settings {
	return &Settings{
		Host:              "localhost",
		Port:              5432,
		Name:              "postgres",
		User:              "postgres",
		Pass:              "postgres",
		SslMode:           "verify-full",
		SslCaCrt:          nil,
		SslDevMode:        false,
		MaxConns:          10,
		MinConns:          2,
		MaxConnLifetime:   time.Hour,
		HealthCheckPeriod: time.Minute,
		QueryTimeout:      5 * time.Second,
	}
}
