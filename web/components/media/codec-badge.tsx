import { Badge } from "@/components/ui/badge";
import { CODEC_BORDER, CODEC_COLORS } from "@/lib/codec";

/**
 * CodecBadge renders a source video codec as a colour-coded pill.
 *
 * When `codec` is null, candidate-focused surfaces hide the badge entirely
 * while the full library shows an explicit "unknown" pill — toggle that with
 * `showUnknown`.
 */
export function CodecBadge({
  codec,
  showUnknown = false,
}: {
  codec: string | null;
  showUnknown?: boolean;
}) {
  if (!codec) {
    if (!showUnknown) return null;
    return (
      <Badge
        variant="outline"
        className="font-mono text-[0.7rem] rounded-[7px]"
      >
        unknown
      </Badge>
    );
  }
  const c = codec.toLowerCase();
  return (
    <Badge
      className={`font-mono text-[0.7rem] rounded-[7px] font-semibold ${CODEC_COLORS[c] ?? "text-slate"} ${CODEC_BORDER[c] ?? "border-line bg-surface-3"}`}
    >
      {codec}
    </Badge>
  );
}
