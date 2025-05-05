package postgres

import (
	"crypto/x509"
	"time"

	utils "github.com/gxmmx/compage-go/utils"
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

type ConfigMap struct {
	Host              string
	Port              string
	Name              string
	User              string
	Pass              string
	SslMode           string
	SslCaCrt          string
	SslDevMode        string
	MaxConns          string
	MinConns          string
	MaxConnLifetime   string
	HealthCheckPeriod string
	QueryTimeout      string
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

func NewConfigMap() *ConfigMap {
	return &ConfigMap{}
}

func (c *Controller) compileConfig() {
	if c.configmap.Host != "" {
		c.settings.Host = c.Unit.GetConfigString(c.configmap.Host)
	}
	if c.configmap.Port != "" {
		c.settings.Port = c.Unit.GetConfigInt(c.configmap.Port)
	}
	if c.configmap.Name != "" {
		c.settings.Name = c.Unit.GetConfigString(c.configmap.Name)
	}
	if c.configmap.User != "" {
		c.settings.User = c.Unit.GetConfigString(c.configmap.User)
	}
	if c.configmap.Pass != "" {
		c.settings.Pass = c.Unit.GetConfigString(c.configmap.Pass)
	}
	if c.configmap.SslMode != "" {
		c.settings.SslMode = c.Unit.GetConfigString(c.configmap.SslMode)
	}
	if c.configmap.SslCaCrt != "" {
		var crt *x509.Certificate
		crtstr := c.Unit.GetConfigString(c.configmap.SslCaCrt)
		crt, err := utils.ParseCrtFromString(crtstr)
		if err != nil {
			crt, _ = utils.ParseCrtFromFile(crtstr)
		}
		if crt != nil {
			c.settings.SslCaCrt = crt
		}
	}
	if c.configmap.SslDevMode != "" {
		c.settings.SslDevMode = c.Unit.GetConfigBool(c.configmap.SslDevMode)
	}
	if c.configmap.MaxConns != "" {
		c.settings.MaxConns = c.Unit.GetConfigInt(c.configmap.MaxConns)
	}
	if c.configmap.MinConns != "" {
		c.settings.MinConns = c.Unit.GetConfigInt(c.configmap.MinConns)
	}
	if c.configmap.MaxConnLifetime != "" {
		maxConnSec := c.Unit.GetConfigInt(c.configmap.MaxConnLifetime)
		c.settings.MaxConnLifetime = time.Duration(maxConnSec) * time.Second
	}
	if c.configmap.HealthCheckPeriod != "" {
		healthCheckSec := c.Unit.GetConfigInt(c.configmap.HealthCheckPeriod)
		c.settings.HealthCheckPeriod = time.Duration(healthCheckSec) * time.Second
	}
	if c.configmap.QueryTimeout != "" {
		queryTimeoutSec := c.Unit.GetConfigInt(c.configmap.QueryTimeout)
		c.settings.QueryTimeout = time.Duration(queryTimeoutSec) * time.Second
	}
}
