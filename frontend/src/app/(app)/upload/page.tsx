import type { Metadata } from "next";
import { UploadStudio } from "@/templates/v1/views/upload/UploadStudio";

export const metadata: Metadata = { title: "Upload" };

/** /upload — media vertical slice: upload → transcode → HLS playback. */
export default function UploadPage() {
  return <UploadStudio />;
}
