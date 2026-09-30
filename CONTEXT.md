# Portal

A self-hosted personal "life OS": one account's media library (movies, music, stories,
comics), finance ledger, journal, people and notifications, in a Go modular monolith
behind a Next.js shell. This glossary holds the vocabulary the code and documents must
share; it is not a spec. Started 2026-09-12 during the journal-attachments design; terms
are added as they are resolved, not in bulk.

## Language

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
