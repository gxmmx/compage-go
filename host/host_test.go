package host

import "testing"

func TestPlatform(t *testing.T) {
	t.Parallel()

	got := platform("linux", "arm64")
	want := PlatformInfo{OS: Linux, Arch: "arm64"}
	if got != want {
		t.Fatalf("platform() = %#v, want %#v", got, want)
	}
}

func TestOSIsUnix(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		os   OS
		want bool
	}{
		{name: "linux", os: Linux, want: true},
		{name: "darwin", os: Darwin, want: true},
		{name: "windows", os: Windows, want: false},
		{name: "other", os: "plan9", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.os.IsUnix(); got != test.want {
				t.Errorf("IsUnix() = %v, want %v", got, test.want)
			}
		})
	}
}
