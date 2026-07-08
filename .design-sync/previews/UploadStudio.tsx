import { UploadStudio } from "portal-frontend";

// Upload → transcode → playback studio (v1 media slice). Idle state: a card with
// the dashed "Choose a video file" dropzone. On pick it uploads, polls the
// worker, then plays the HLS output with Vidstack. In a static preview there is
// no API, so the library fetch no-ops and only the upload card shows — the
// honest idle state. Wrapped in a padded page surface.

export const Studio = () => (
  <div style={{ background: "var(--tpl-body, #f5f6fa)", padding: 32 }}>
    <UploadStudio />
  </div>
);
