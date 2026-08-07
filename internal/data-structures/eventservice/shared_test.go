package eventservice

import "testing"

func TestSafeReorgBlock(t *testing.T) {
	cases := []struct {
		latest, reorgDepth, want uint64
	}{
		{latest: 1000, reorgDepth: 32, want: 969},
		{latest: 31, reorgDepth: 32, want: 0},
		{latest: 0, reorgDepth: 32, want: 0},
		{latest: 32, reorgDepth: 32, want: 1},
	}
	for _, c := range cases {
		if got := safeReorgBlock(c.latest, c.reorgDepth); got != c.want {
			t.Errorf("safeReorgBlock(%d, %d) = %d, want %d", c.latest, c.reorgDepth, got, c.want)
		}
	}
}
