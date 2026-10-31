package app

import "testing"

func TestNormalize(t *testing.T) {
	if a, b := normalize("Ctrl+Alt+Cmd+C"), normalize("command+option+control+c"); a != b {
		t.Errorf("%q != %q", a, b)
	}
	if a, b := normalize("Ctrl+C"), normalize("Ctrl+Shift+C"); a == b {
		t.Errorf("distinct accelerators normalized to %q", a)
	}
	if got := normalize("Cmd++"); got != "cmd++" {
		t.Errorf("normalize(Cmd++) = %q", got)
	}
}
