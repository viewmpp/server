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

func TestKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"signin:a@b.c", "signin"},
		{"signup:someone@example.com", "signup"},
		{"reset:person@corp.co.uk", "reset"},
		{"unlock:ZPSSGoGtjnlbTkjN", "unlock"},
		{"unlock-ip:203.0.113.5", "unlock-ip"},
		{"upload-user:4217", "upload-user"},
		{"export-ip:2001:db8::1", "export-ip"},
		{"plain", "plain"},
		{"", ""},
	}

	for _, c := range cases {
		if got := Key(c.in); got != c.want {
			t.Errorf("Key(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
