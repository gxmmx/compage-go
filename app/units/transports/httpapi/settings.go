package httpapi

import (
	"crypto"
	"crypto/x509"

	utils "github.com/gxmmx/compage-go/utils"
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
	SslCrt     []*x509.Certificate
	SslKey     crypto.PrivateKey
	SslCaCrt   *x509.Certificate
}

type ConfigMap struct {
	Host           string
	Port           string
	Prefix         string
	SwaggerEnabled string
	SwaggerPath    string
	SslEnabled     string
	SslCrt         string
	SslKey         string
	SslCaCrt       string
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
	if c.configmap.Prefix != "" {
		c.settings.Prefix = c.Unit.GetConfigString(c.configmap.Prefix)
	}
	if c.configmap.SwaggerEnabled != "" {
		c.settings.SwaggerEnabled = c.Unit.GetConfigBool(c.configmap.SwaggerEnabled)
	}
	if c.configmap.SwaggerPath != "" {
		c.settings.SwaggerPath = c.Unit.GetConfigString(c.configmap.SwaggerPath)
	}
	if c.configmap.SslEnabled != "" {
		c.settings.SslEnabled = c.Unit.GetConfigBool(c.configmap.SslEnabled)
	}
	var crts []*x509.Certificate
	var keys []crypto.PrivateKey
	var cerr error
	if c.configmap.SslCrt != "" {
		crtstr := c.Unit.GetConfigString(c.configmap.SslCrt)
		crts, keys, cerr = utils.ParseCrtBundleFromString(crtstr)
		if cerr != nil {
			crts, keys, _ = utils.ParseCrtBundleFromFile(crtstr)
		}
		if len(crts) > 0 {
			c.settings.SslCrt = crts
		}
		if len(keys) > 0 {
			c.settings.SslKey = keys[0]
		}
	}
	if c.configmap.SslKey != "" {
		keystr := c.Unit.GetConfigString(c.configmap.SslKey)
		key, err := utils.ParseKeyFromString(keystr)
		if err != nil {
			key, _ = utils.ParseKeyFromFile(keystr)
		}
		if key != nil {
			c.settings.SslKey = key
		}
	}
	if c.configmap.SslCaCrt != "" {
		crtstr := c.Unit.GetConfigString(c.configmap.SslCaCrt)
		crt, err := utils.ParseCrtFromString(crtstr)
		if err != nil {
			crt, _ = utils.ParseCrtFromFile(crtstr)
		}
		if crt != nil {
			c.settings.SslCaCrt = crt
		}
	}
}
