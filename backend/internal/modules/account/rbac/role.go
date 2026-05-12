package rbac

import (
	"fmt"

	"github.com/google/uuid"
)

// Role is the in-memory projection of a row in `roles`.
type Role struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	ParentID    *uuid.UUID
	IsSystem    bool
}

// Catalog indexes the full role table for hierarchy walks. Build once per
// permission-cache miss, not per request.
type Catalog struct {
	byID   map[uuid.UUID]Role
	byCode map[string]Role
}

func NewCatalog(roles []Role) (*Catalog, error) {
	c := &Catalog{
		byID:   make(map[uuid.UUID]Role, len(roles)),
		byCode: make(map[string]Role, len(roles)),
	}
	for _, r := range roles {
		if _, dup := c.byID[r.ID]; dup {
			return nil, fmt.Errorf("rbac: duplicate role id %s", r.ID)
		}
		c.byID[r.ID] = r
		c.byCode[r.Code] = r
	}
	if err := c.detectCycles(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalog) ByCode(code string) (Role, bool) {
	r, ok := c.byCode[code]
	return r, ok
}

func (c *Catalog) ByID(id uuid.UUID) (Role, bool) {
	r, ok := c.byID[id]
	return r, ok
}

// Ancestors returns the role plus every transitive parent, deduplicated.
// Order is stable (deterministic walk from leaf upward).
func (c *Catalog) Ancestors(id uuid.UUID) []Role {
	var out []Role
	seen := make(map[uuid.UUID]struct{})
	cur := id
	for {
		if _, dup := seen[cur]; dup {
			return out // cycle guard at lookup time
		}
		seen[cur] = struct{}{}
		r, ok := c.byID[cur]
		if !ok {
			return out
		}
		out = append(out, r)
		if r.ParentID == nil {
			return out
		}
		cur = *r.ParentID
	}
}

// detectCycles walks every role's ancestry. With N roles, at most N hops are
// allowed before we declare a cycle.
func (c *Catalog) detectCycles() error {
	limit := len(c.byID) + 1
	for id, root := range c.byID {
		seen := make(map[uuid.UUID]struct{})
		cur := id
		for hops := 0; hops <= limit; hops++ {
			if _, dup := seen[cur]; dup {
				return fmt.Errorf("rbac: cycle detected involving role %s", root.Code)
			}
			seen[cur] = struct{}{}
			r := c.byID[cur]
			if r.ParentID == nil {
				break
			}
			cur = *r.ParentID
		}
	}
	return nil
}
