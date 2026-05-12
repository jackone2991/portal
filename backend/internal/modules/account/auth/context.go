package auth

import "context"

type ctxKey struct{}

// WithIdentity returns ctx with the verified Identity attached.
// Called only by the auth middleware.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the Identity bound to ctx by the auth middleware.
// Returns (nil, false) for anonymous requests.
func FromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(*Identity)
	if !ok || id == nil {
		return nil, false
	}
	return id, true
}

// MustFromContext is for handlers downstream of RequireAuth where the
// identity is guaranteed. Panics if missing — surfaces a routing bug fast.
func MustFromContext(ctx context.Context) *Identity {
	id, ok := FromContext(ctx)
	if !ok {
		panic("auth: identity missing from context — wire RequireAuth middleware")
	}
	return id
}
