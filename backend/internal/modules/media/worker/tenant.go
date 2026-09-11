package worker

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// RunInTenant opens a transaction with app.current_tenant pinned to the given
// user's personal org and runs fn inside it. cmd/worker supplies it; cmd/api
// does not (its requests are already tenant-scoped by RequireTenant).
//
// Kept as a primitive-signature func so this package never imports the media
// package or platform/db — the same no-cycle rule the Repo interface follows.
type RunInTenant func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error

// inTenant runs fn inside the owner's tenant scope.
//
// This is the ADR-07 "increment 1b" the 0020 migration's ⚠️ gate was waiting on.
// Every worker write to a tenant-scoped table (media_asset_variants, assets)
// must go through it: those tables carry
// `tenant_id DEFAULT current_setting('app.current_tenant')::uuid`, which ERRORS
// when the GUC is unset — and a column-DEFAULT error is not bypassed even by the
// superuser.
//
// A nil run is the pre-cutover wiring (cmd/api, and tests): fn runs unscoped,
// which is correct only while DATABASE_URL is still the superuser. Once run is
// supplied, a task without a usable owner is a hard error rather than a silent
// unscoped write — the failure mode this whole change exists to prevent.
func inTenant(ctx context.Context, run RunInTenant, taskName, owner string, fn func(context.Context) error) error {
	if run == nil {
		return fn(ctx)
	}
	uid, err := uuid.Parse(owner)
	if err != nil {
		return fmt.Errorf("%s: owner_user_id %q is not a uuid — cannot open a tenant scope", taskName, owner)
	}
	return run(ctx, uid, fn)
}
