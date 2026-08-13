package safelog

import "testing"

func TestURI(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/reset/ABCDEF123456", "/reset/[redacted]"},
		{"/reset/ABCDEF?x=1", "/reset/[redacted]"},
		{"/reset", "/reset"},
		{"/reset/", "/reset/"},
		{"/p/abc", "/p/abc"},
		{"/api/v1/upload?name=plan.mpp", "/api/v1/upload"},
	}

	for _, c := range cases {
		if got := URI(c.in); got != c.want {
			t.Errorf("URI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
