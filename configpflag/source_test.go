package configpflag

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestSource(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("name", "default", "")
	source := Source{Flags: flags}
	if _, changed, found := source.Lookup("name"); !found || changed {
		t.Fatalf("unchanged lookup = found:%v changed:%v", found, changed)
	}
	if err := flags.Set("name", "set"); err != nil {
		t.Fatal(err)
	}
	value, changed, found := source.Lookup("name")
	if !found || !changed || value != "set" {
		t.Fatalf("lookup = %q %v %v", value, changed, found)
	}
}
