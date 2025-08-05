package input

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	cmperr "github.com/gxmmx/compage-go/errors"
	cmplog "github.com/gxmmx/compage-go/logger"
	cmpstl "github.com/gxmmx/compage-go/style"

	"golang.org/x/term"
)

// -----------------------------------------------------------------------------
// Controllers
// -----------------------------------------------------------------------------

// Package controller implements the Input interface
type Controller struct {
	msg            string
	indent         int
	pColor         string
	writer         io.Writer
	reader         io.Reader
	bufReader      *bufio.Reader
	secretReader   *os.File
	isTTY          bool
	validationFunc Validator
	validationMsg  string
	defaultValue   string
	secret         bool
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

// Creates a new Input instance with default values and options.
func New(opts ...Option) *Controller {
	ctl := &Controller{
		indent:       0,
		pColor:       "",
		writer:       defaultWriter,
		reader:       defaultReader,
		secretReader: defaultReader,
		bufReader:    bufio.NewReader(defaultReader),
		isTTY:        term.IsTerminal(int(os.Stdin.Fd())),
	}

	for _, opt := range opts {
		opt(ctl)
	}

	return ctl
}

// Shorthand constructor for creating input with a simple prompt.
func Prompt(msg string, opts ...Option) (string, error) {
	input := New(opts...)
	return input.Prompt(msg)
}

// -----------------------------------------------------------------------------
// Controller methods
// -----------------------------------------------------------------------------

// Create a prompt with the given message and return the input string.
func (ctl *Controller) Prompt(msg string) (string, error) {
	return ctl.prompt(msg, "")
}

func (ctl *Controller) PromptMulti(msg string) ([]string, error) {
	var lines []string
	indentation := strings.Repeat(cmplog.IndentString, ctl.indent)
	firstprompt := cmpstl.Color(ctl.pColor).Apply(indentation + msg)
	fmt.Fprintln(ctl.writer, firstprompt)
	for {
		line, err := ctl.prompt("", "- ")
		if err != nil {
			return lines, err
		}
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func (ctl *Controller) PromptUntil(msg string) (string, error) {
	for {
		input, err := ctl.Prompt(msg)
		if err != nil {
			if cmperr.IsOfKind(err, cmperr.KindValidationFailed) {
				indentation := strings.Repeat(cmplog.IndentString, ctl.indent)
				prompt := cmpstl.Red().Apply(indentation + err.Error())
				fmt.Fprintln(ctl.writer, prompt)
				continue
			}
			return "", err
		}
		return input, nil
	}
}

func (ctl *Controller) Confirm(msg string) (bool, error) {
	for {
		answer, err := ctl.Prompt(msg + " [y/n]")
		if err != nil {
			return false, err
		}

		if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
			return true, nil
		}
		return false, nil
	}
}

// -----------------------------------------------------------------------------
// Controller internal methods
// -----------------------------------------------------------------------------

// Internal method to handle the actual prompting logic.
func (ctl *Controller) prompt(msg string, overwritemsg string) (string, error) {
	if !ctl.isTTY {
		return "", cmperr.NewUnavailable("Prompting is not available in non terminal environment", nil)
	}

	indentation := strings.Repeat(cmplog.IndentString, ctl.indent)
	prompt := indentation + msg
	if overwritemsg != "" {
		prompt = indentation + overwritemsg
	} else {
		if ctl.defaultValue != "" {
			prompt += fmt.Sprintf(" [%s]: ", ctl.defaultValue)
		} else {
			prompt += ": "
		}
	}
	if ctl.pColor != "" {
		prompt = cmpstl.Color(ctl.pColor).Apply(prompt)
	}

	fmt.Fprint(ctl.writer, prompt)

	var line string
	var err error
	if ctl.secret {
		byteLine, readErr := readPasswordFn(int(ctl.secretReader.Fd()))
		fmt.Fprintln(ctl.writer) // move to next line after secret input
		line = string(byteLine)
		err = readErr
	} else {
		line, err = ctl.bufReader.ReadString('\n')
	}
	if err != nil {
		return "", cmperr.NewInvalidInput("Failed to read input", err)
	}
	line = strings.TrimSpace(line)

	if line == "" && ctl.defaultValue != "" {
		line = ctl.defaultValue
	}

	if ctl.validationFunc != nil && !ctl.validationFunc(line) {
		return "", cmperr.NewValidationFailed(ctl.validationMsg, nil)
	}
	return line, nil
}
