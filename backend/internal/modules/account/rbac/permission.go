// Package rbac implements hierarchical RBAC with wildcard permissions.
//
// Permission grammar:
//
//	<resource>:<action>[:<scope>]   e.g.  movies:read, assets:delete:own
//	*                                — superadmin wildcard
//	<resource>:*                     — all actions on a resource
//	*:<action>                       — all resources, one action
//
// Matching is segment-aligned (not glob): a granted permission matches a
// required permission if every segment of the granted is either equal or '*',
// and the segment counts agree (with the implicit-scope rule below).
//
// Implicit scope: a 2-segment grant ("movies:write") satisfies a 3-segment
// require ("movies:write:any") only when the third segment is "any". Owners
// must be explicitly granted ":own" and check ownership at call site.
package rbac

import (
	"fmt"
	"strings"
)

// Permission is the parsed form of a permission code.
// Use Parse to construct; the zero value is invalid.
type Permission struct {
	Resource string
	Action   string
	Scope    string // "" if none specified
}

// Wildcard returns true when the entire permission is '*'.
func (p Permission) Wildcard() bool {
	return p.Resource == "*" && p.Action == "" && p.Scope == ""
}

// String renders the canonical form of the permission.
func (p Permission) String() string {
	if p.Wildcard() {
		return "*"
	}
	if p.Scope == "" {
		return p.Resource + ":" + p.Action
	}
	return p.Resource + ":" + p.Action + ":" + p.Scope
}

// Parse a permission code. Returns an error for malformed input.
//
// Valid examples:
//
//	"*", "movies:read", "assets:delete:own", "movies:*", "*:read"
func Parse(code string) (Permission, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return Permission{}, fmt.Errorf("rbac: empty permission code")
	}
	if code == "*" {
		return Permission{Resource: "*"}, nil
	}
	parts := strings.Split(code, ":")
	switch len(parts) {
	case 2:
		if !validSegment(parts[0]) || !validSegment(parts[1]) {
			return Permission{}, fmt.Errorf("rbac: invalid permission code %q", code)
		}
		return Permission{Resource: parts[0], Action: parts[1]}, nil
	case 3:
		if !validSegment(parts[0]) || !validSegment(parts[1]) || !validSegment(parts[2]) {
			return Permission{}, fmt.Errorf("rbac: invalid permission code %q", code)
		}
		return Permission{Resource: parts[0], Action: parts[1], Scope: parts[2]}, nil
	default:
		return Permission{}, fmt.Errorf("rbac: permission %q must have 2 or 3 segments", code)
	}
}

// MustParse panics on error. Use only for static permission constants in code.
func MustParse(code string) Permission {
	p, err := Parse(code)
	if err != nil {
		panic(err)
	}
	return p
}

// validSegment enforces the same character class as the DB CHECK constraint.
func validSegment(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '*' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

// Matches reports whether the granted permission satisfies the required permission.
// Wildcards and segment counts follow the rules described in the package doc.
//
//	granted("*")                      satisfies any required
//	granted("movies:*")               satisfies movies:<anything>[:<anything>]
//	granted("*:read")                 satisfies <anything>:read
//	granted("movies:write")           satisfies movies:write     and  movies:write:any
//	granted("movies:write:own")       satisfies movies:write:own ONLY
//	granted("movies:write:any")       satisfies movies:write     and  movies:write:any
func (granted Permission) Matches(required Permission) bool {
	if granted.Wildcard() {
		return true
	}
	if !segMatch(granted.Resource, required.Resource) {
		return false
	}
	if !segMatch(granted.Action, required.Action) {
		return false
	}
	// Scope rules:
	//   granted scope "" ↔ required scope "" or "any"
	//   granted scope "any" ↔ required scope "" or "any"
	//   granted scope "own" ↔ required scope "own"
	//   granted scope "*" ↔ any required scope
	gs, rs := granted.Scope, required.Scope
	switch {
	case gs == "*":
		return true
	case gs == "" && (rs == "" || rs == "any"):
		return true
	case gs == "any" && (rs == "" || rs == "any"):
		return true
	case gs == "own" && rs == "own":
		return true
	case gs == rs && gs != "":
		return true
	}
	return false
}

func segMatch(granted, required string) bool {
	if granted == "*" {
		return true
	}
	return granted == required
}

// Set is an effective permission set computed for a single principal.
// Lookups are O(N) — N is small (single-digit to low-double-digit).
//
// Use NewSet to construct; do not mutate after creation (cached values may
// share the underlying slice).
type Set struct {
	codes []Permission
}

// NewSet builds a Set from raw permission codes. Invalid codes are silently
// skipped (the DB CHECK constraint should prevent any from reaching here);
// callers wanting strictness should pre-validate via Parse.
func NewSet(codes []string) Set {
	out := make([]Permission, 0, len(codes))
	for _, c := range codes {
		if p, err := Parse(c); err == nil {
			out = append(out, p)
		}
	}
	return Set{codes: out}
}

// Allows reports whether the set grants the given required permission.
func (s Set) Allows(required Permission) bool {
	for _, granted := range s.codes {
		if granted.Matches(required) {
			return true
		}
	}
	return false
}

// AllowsCode is a convenience for the common case of checking against a code.
// Returns false on malformed required code (fail-closed).
func (s Set) AllowsCode(code string) bool {
	required, err := Parse(code)
	if err != nil {
		return false
	}
	return s.Allows(required)
}

// Codes returns the canonical string form of every permission in the set.
// Order is stable across calls.
func (s Set) Codes() []string {
	out := make([]string, len(s.codes))
	for i, p := range s.codes {
		out[i] = p.String()
	}
	return out
}
