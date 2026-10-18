package client

import "testing"

func TestSplitDetach(t *testing.T) {
	cases := []struct {
		name     string
		chunk    []byte
		forward  string
		detached bool
	}{
		{"plain bytes pass through", []byte("abc"), "abc", false},
		{"key splits the chunk", []byte("ab\x1cbd"), "ab", true},
		{"key alone forwards nothing", []byte("\x1c"), "", true},
		{"empty chunk", nil, "", false},
		{"key at the end", []byte("ab\x1c"), "ab", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forward, detached := splitDetach(tc.chunk)
			if string(forward) != tc.forward {
				t.Errorf("forward = %q, want %q", forward, tc.forward)
			}
			if detached != tc.detached {
				t.Errorf("detached = %v, want %v", detached, tc.detached)
			}
		})
	}
}
