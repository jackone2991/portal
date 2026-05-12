package rbac

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in     string
		ok     bool
		canon  string
	}{
		{"*", true, "*"},
		{"movies:read", true, "movies:read"},
		{"assets:delete:own", true, "assets:delete:own"},
		{"movies:*", true, "movies:*"},
		{"*:read", true, "*:read"},
		{"", false, ""},
		{"movies", false, ""},
		{"movies:read:any:extra", false, ""},
		{"Movies:read", false, ""},                   // uppercase
		{"movies:read with space", false, ""},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if c.ok {
			if err != nil {
				t.Errorf("Parse(%q) unexpected err: %v", c.in, err)
				continue
			}
			if got.String() != c.canon {
				t.Errorf("Parse(%q).String() = %q, want %q", c.in, got.String(), c.canon)
			}
		} else if err == nil {
			t.Errorf("Parse(%q) expected error, got %+v", c.in, got)
		}
	}
}

func TestMatches(t *testing.T) {
	cases := []struct {
		granted, required string
		want              bool
	}{
		// Superadmin wildcard
		{"*", "movies:read", true},
		{"*", "anything:goes:here", true},

		// Resource wildcard
		{"movies:*", "movies:read", true},
		{"movies:*", "movies:write:own", true},
		{"movies:*", "music:read", false},

		// Action wildcard
		{"*:read", "movies:read", true},
		{"*:read", "music:read", true},
		{"*:read", "movies:write", false},

		// Scope rules: 2-segment grant covers ":any" and bare requireds
		{"movies:write", "movies:write", true},
		{"movies:write", "movies:write:any", true},
		{"movies:write", "movies:write:own", false},

		// 3-segment grant of ":any" covers bare required
		{"movies:write:any", "movies:write", true},
		{"movies:write:any", "movies:write:any", true},
		{"movies:write:any", "movies:write:own", false},

		// :own only matches :own
		{"assets:delete:own", "assets:delete:own", true},
		{"assets:delete:own", "assets:delete:any", false},
		{"assets:delete:own", "assets:delete", false},

		// Mismatches
		{"movies:read", "music:read", false},
		{"movies:read", "movies:write", false},
	}
	for _, c := range cases {
		g := MustParse(c.granted)
		r := MustParse(c.required)
		if got := g.Matches(r); got != c.want {
			t.Errorf("granted %q matches required %q = %v, want %v", c.granted, c.required, got, c.want)
		}
	}
}

func TestSetAllows(t *testing.T) {
	s := NewSet([]string{"movies:*", "assets:read:own", "comments:write"})

	allow := []string{"movies:read", "movies:write", "movies:publish", "assets:read:own", "comments:write"}
	for _, c := range allow {
		if !s.AllowsCode(c) {
			t.Errorf("expected set to allow %q", c)
		}
	}
	deny := []string{"music:read", "assets:read:any", "users:write:any", "*"}
	for _, c := range deny {
		if s.AllowsCode(c) {
			t.Errorf("expected set to deny %q", c)
		}
	}
}

func TestSetAllowsMalformedDenied(t *testing.T) {
	s := NewSet([]string{"*"})
	if s.AllowsCode("not a perm") {
		t.Errorf("malformed code should fail closed even with superadmin grant")
	}
}
