package configpflag

import (
	"github.com/gxmmx/compage-go/config"
	"github.com/spf13/pflag"
)

// Source exposes a pflag FlagSet without making config depend on pflag. Its
// StringSlice values use pflag's string representation, which config decodes.
type Source struct{ Flags *pflag.FlagSet }

// Lookup implements config.FlagSource.
func (s Source) Lookup(name string) (value string, changed bool, found bool) {
	if s.Flags == nil {
		return "", false, false
	}
	f := s.Flags.Lookup(name)
	if f == nil {
		return "", false, false
	}
	return f.Value.String(), f.Changed, true
}

var _ config.FlagSource = Source{}
