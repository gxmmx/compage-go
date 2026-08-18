package configpflag

import (
	"reflect"
	"testing"

	"github.com/gxmmx/compage-go/config"
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

func TestSourceStringSliceWithConfig(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.StringSlice("tags", nil, "")
	if err := flags.Set("tags", "one,two"); err != nil {
		t.Fatal(err)
	}
	if err := flags.Set("tags", `"three,four"`); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load[struct {
		Tags []string `flag:"tags"`
	}](config.WithFlagSource(Source{Flags: flags}))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.Values().Tags, []string{"one", "two", "three,four"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Tags = %#v, want %#v", got, want)
	}
	if source, ok := c.Source("tags"); !ok || source != config.SourceFlag {
		t.Fatalf("Source(tags) = %v, %v", source, ok)
	}
}
