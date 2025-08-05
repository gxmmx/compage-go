package spinner

import (
	"os"
	"time"
)

// -----------------------------------------------------------------------------
// Defaults
// -----------------------------------------------------------------------------

// Default delay between spinner frames
var defaultDelay = 100 * time.Millisecond

// Default writer for output
var defaultWriter = os.Stdout

// Default spinner colors
var defaultSpinnerColor = "green"
var defaultMessageColor = ""

// -----------------------------------------------------------------------------
// Variables
// -----------------------------------------------------------------------------

// Spinner frames for animation
var frames = []rune{'\u280B', '\u2819', '\u2839', '\u2838', '\u283C', '\u2834', '\u2826', '\u2827', '\u2807', '\u280F'}
