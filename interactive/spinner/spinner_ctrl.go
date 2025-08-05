package spinner

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	cmplog "github.com/gxmmx/compage-go/logger"
	cmpstl "github.com/gxmmx/compage-go/style"

	"golang.org/x/term"
)

// -----------------------------------------------------------------------------
// Controllers
// -----------------------------------------------------------------------------

type Controller struct {
	msg    string
	indent int
	prefix string
	sColor string
	mColor string
	delay  time.Duration
	writer io.Writer
	done   chan struct{}
	mutex  *sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	isTTY  bool
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

// Start creates and runs a spinner with the given message and delay
func Start(msg string, opts ...Option) Spinner {
	ctx, cancel := context.WithCancel(context.Background())
	ctl := &Controller{
		msg:    msg,
		indent: 0,
		prefix: "",
		sColor: defaultSpinnerColor,
		mColor: defaultMessageColor,
		delay:  defaultDelay,
		writer: defaultWriter,
		done:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
		isTTY:  term.IsTerminal(int(os.Stdout.Fd())),
	}

	// Apply options
	for _, opt := range opts {
		opt(ctl)
	}

	// Set indent
	if ctl.indent > 0 {
		ctl.prefix = fmt.Sprintf("%s", strings.Repeat(cmplog.IndentString, ctl.indent))
	}

	// Set mutex if not provided
	if ctl.mutex == nil {
		ctl.mutex = &sync.Mutex{}
	}

	// Hide cursor
	if ctl.isTTY {
		ctl.mutex.Lock()
		fmt.Fprint(ctl.writer, "\033[?25l")
		ctl.mutex.Unlock()
	}

	go ctl.loop()
	return ctl
}

// -----------------------------------------------------------------------------
// Controller methods
// -----------------------------------------------------------------------------

// Stop the spinner and optionally print a final message
func (ctl *Controller) Stop(final string) {
	ctl.cancel()
	<-ctl.done
	if !ctl.isTTY {
		return
	}
	if final != "" {
		ctl.mutex.Lock()
		fmt.Fprintln(ctl.writer, ctl.prefix+final)
		ctl.mutex.Unlock()
	}
}

// -----------------------------------------------------------------------------
// Controller internal methods
// -----------------------------------------------------------------------------

func (ctl *Controller) loop() {
	ticker := time.NewTicker(ctl.delay)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-ctl.ctx.Done():
			if ctl.isTTY {
				ctl.clearLine()
				// Show cursor again
				ctl.mutex.Lock()
				fmt.Fprint(ctl.writer, "\033[?25h")
				ctl.mutex.Unlock()
			}
			close(ctl.done)
			return
		case <-ticker.C:
			if ctl.isTTY {
				ctl.mutex.Lock()
				fmt.Fprintf(ctl.writer, "\r%s%s %s", ctl.prefix, cmpstl.Color(ctl.sColor).Apply(string(frames[i%len(frames)])), cmpstl.Color(ctl.mColor).Apply(ctl.msg))
				ctl.mutex.Unlock()
				i++
			}
		}
	}
}

func (ctl *Controller) clearLine() {
	ctl.mutex.Lock()
	defer ctl.mutex.Unlock()
	fmt.Fprint(ctl.writer, "\r\033[K")
}
