import { Badge } from "@/components/ui/badge";
import type { MediaFile } from "@/lib/api";

export function OversizeBadge({ file }: { file: MediaFile }) {
  if (!file.is_oversized) return null;
  return (
    <Badge
      className="text-xs rounded-lg font-semibold text-gold border-gold/32 bg-gold/10 shrink-0"
      title={`${file.oversize_ratio.toFixed(1)}× the expected bitrate for this codec and resolution`}
    >
      Oversized · {file.oversize_ratio.toFixed(1)}×
    </Badge>
  );
}
