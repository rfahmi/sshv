package ssh

import "testing"

func TestContainsPasswordPrompt(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"root@10.0.0.1's password: ", true},
		{"Password:", true},
		{"Last login: Fri Nov 27", false},
	}
	for _, tc := range cases {
		if got := containsPasswordPrompt([]byte(tc.in)); got != tc.want {
			t.Errorf("containsPasswordPrompt(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
