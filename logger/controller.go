package logger

import "fmt"

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Logger interface {
	Info(msg string)
}

type Parser interface {
	Level(string)
	TypeCli()
	TypeApp()

	Parse() (Logger, error)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	logLevel string
	logType  string

	logger string
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewLogger() *Controller {
	return &Controller{
		logLevel: "info",
		logType:  "cli",
	}
}

// -----------------------------------------------------------------------------
// Parser methods
// -----------------------------------------------------------------------------

func (c *Controller) Level(level string) {
	c.logLevel = level
}

func (c *Controller) TypeCli() {
	c.logType = "cli"
}

func (c *Controller) TypeApp() {
	c.logType = "app"
}

func (c *Controller) Parse() (Logger, error) {
	// Here we would normally set up the logger based on the level and type
	// For simplicity, we just return a simple logger implementation
	return c, nil
}

// -----------------------------------------------------------------------------
// Logger methods
// -----------------------------------------------------------------------------

func (c *Controller) Info(msg string) {
	fmt.Println(c.logType, ":", msg)
}
