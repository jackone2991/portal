# Portal

A self-hosted personal "life OS": one account's media library (movies, music, stories,
comics), finance ledger, journal, people and notifications, in a Go modular monolith
behind a Next.js shell. This glossary holds the vocabulary the code and documents must
share; it is not a spec. Started 2026-09-12 during the journal-attachments design; terms
are added as they are resolved, not in bulk.

## Language

### Identity

**User**:
One person who can sign in to the instance: an email, a password, an Approval state and
roles. Everything a person owns in Portal belongs to their User.
_Avoid_: account (reserved for a ledger Account), member, profile

**Session**:
One signed-in device or browser of a User. Logging out ends that Session only; "log out
everywhere" ends all of them.
_Avoid_: login, token, device (as a synonym)

**Approval**:
The one-time admission decision on a User: Pending until decided, then Approved or
Rejected. Only an Approved User can hold a Session.
_Avoid_: verification, activation, enabled

**Rejected**:
The Approval state of a User who was refused admission. A Rejected User cannot register
again with the same email; only an admin can move them back to Pending or to Approved.
_Avoid_: banned, blocked, deleted

**Disabled**:
A temporary, reversible suspension of an Approved User: they cannot hold a Session until
an admin enables them again. Not part of Approval — a Disabled User is still Approved.
_Avoid_: suspended, revoked, rejected

**Superadmin**:
A User whose effective permissions include everything (`*`), whichever role grants it.
Defined by permission, not by role name.
_Avoid_: owner, root, admin (admin is only a role name)

**Approver**:
A User whose effective permissions allow deciding Approval (`users:approve`). Out of the
box only Superadmins are Approvers.
_Avoid_: moderator, reviewer

### Journal

**Entry**:
One dated journal record written by one user: a markdown body, an optional mood, when it
happened (`occurred_at`), an optional Location, and its Attachments. The body is plain
markdown and carries no structure of its own; it may be empty only when the Entry has at
least one Attachment — an Entry is text, or a picture, or both, never a Location alone.
_Avoid_: post, note, journal item

**Attachment**:
A reference from an Entry to a media Asset that the Entry shows. An Attachment is not the
Asset — the Asset has its own owner, lifecycle and visibility in the media module; the
Attachment only says "this Entry shows it".
_Avoid_: photo (as a noun for the reference), image link, embed, asset_id (in prose)

**Location**:
Where an Entry happened: a name and coordinates, stored on the Entry. A property of the
Entry, not an Attachment, and not a reference to anything in another module.
_Avoid_: place, geo, pin, attachment (for a Location)

### Media

**Asset**:
One uploaded media object owned by one user — an image, video or audio file — with a
processing status and derived variants, managed by the media module. Other modules refer
to Assets by id and never own them.
_Avoid_: file, upload, photo (as a synonym for the object)

### Tenancy

**Group**:
A named set of members of one tenant, kept by the tenant module and managed by the
tenant's owner. A tenant can hold several Groups — one household each, say — and a member
can be in more than one. A Group grants no permission and does not decide who can find
whom; it only decides who is Family.
_Avoid_: household (for the Group itself), circle, team, user group (the deferred RBAC
concept)

**Family**:
For one item, the Users who share at least one Group with its owner in the tenant the item
lives in. Family read the owner's published music, movies and stories; another member of
the same tenant who shares no Group with the owner is not Family.
_Avoid_: household (as the audience), tenant members, friends (friends are accepted
connections, a separate audience)
