#!/bin/sh
# bench-encode.sh — compare Reclaim encode profiles on your own media.
#
# Cuts a short clip from each input file, encodes it with every profile the
# same way the worker does (same encoder, -crf, -preset), and reports wall
# time, speed (× realtime), output size against the source clip, and a quality
# score (VMAF when this ffmpeg has libvmaf, SSIM otherwise). The clips are
# copies; your library files are only ever read.
#
# POSIX sh so it runs inside the Alpine-based Reclaim image, against the exact
# ffmpeg build that will do the real encodes. Pipe it in, no copy needed:
#
#   docker exec -i reclaim sh -s -- \
#     "/movies/Some Movie (2010)/Some Movie (2010).mkv" \
#     "/tv/Some Show/Season 1/S01E01.mkv" < scripts/bench-encode.sh
#
# Options:
#   -p codec:crf:preset   profile to test; repeat for several
#                         (default: hevc:26:medium and av1:30:6, Reclaim's defaults)
#   -d seconds            clip length (default 60)
#   -w dir                scratch directory (default: a new dir under /tmp)
#   -x "args"             extra ffmpeg args appended to every encode, as in a
#                         profile's extra args
#
# Run it while the encode window is closed: a concurrent Reclaim job competes
# for the same CPU and skews every timing.

set -eu

usage() {
	sed -n '2,/^$/s/^# \{0,1\}//p' "$0" 2>/dev/null || true
	echo "usage: bench-encode.sh [-p codec:crf:preset]... [-d seconds] [-w dir] [-x args] file..." >&2
	exit 2
}

profiles=""
clip_len=60
work=""
extra=""
while getopts "p:d:w:x:h" opt; do
	case "$opt" in
	p) profiles="$profiles $OPTARG" ;;
	d) clip_len="$OPTARG" ;;
	w) work="$OPTARG" ;;
	x) extra="$OPTARG" ;;
	*) usage ;;
	esac
done
shift $((OPTIND - 1))
[ $# -gt 0 ] || usage
[ -n "$profiles" ] || profiles="hevc:26:medium av1:30:6"

encoder_for() {
	case "$1" in
	hevc) echo libx265 ;;
	av1) echo libsvtav1 ;;
	*) echo "unknown codec '$1' (expected hevc or av1)" >&2; exit 2 ;;
	esac
}

for p in $profiles; do
	enc=$(encoder_for "${p%%:*}")
	ffmpeg -hide_banner -encoders 2>/dev/null | grep -q " $enc " ||
		{ echo "this ffmpeg has no $enc encoder" >&2; exit 1; }
done

if ffmpeg -hide_banner -filters 2>/dev/null | grep -q " libvmaf "; then
	metric=VMAF
else
	metric=SSIM
fi

if [ -z "$work" ]; then
	work=$(mktemp -d /tmp/reclaim-bench.XXXXXX)
	trap 'rm -rf "$work"' EXIT INT TERM
else
	mkdir -p "$work"
fi
results="$work/results.tsv"
: >"$results"

duration_of() {
	ffprobe -v error -show_entries format=duration -of default=nw=1:nk=1 "$1"
}

size_of() {
	wc -c <"$1" | tr -d ' '
}

# quality_of distorted reference — prints the score on a 0–100 scale.
quality_of() {
	if [ "$metric" = VMAF ]; then
		ffmpeg -nostdin -hide_banner -i "$1" -i "$2" -lavfi \
			"[0:v]format=yuv420p[d];[1:v]format=yuv420p[r];[d][r]libvmaf" \
			-f null - 2>&1 | sed -n 's/.*VMAF score: \([0-9.]*\).*/\1/p' | tail -n 1
	else
		ffmpeg -nostdin -hide_banner -i "$1" -i "$2" -lavfi \
			"[0:v]format=yuv420p[d];[1:v]format=yuv420p[r];[d][r]ssim" \
			-f null - 2>&1 | sed -n 's/.*SSIM .* All:\([0-9.]*\).*/\1/p' | tail -n 1 |
			awk '{ printf "%.2f", $1 * 100 }'
	fi
}

n=0
for src in "$@"; do
	n=$((n + 1))
	if [ ! -r "$src" ]; then
		echo "skip: cannot read $src" >&2
		continue
	fi
	total=$(duration_of "$src")
	# Start 20% in: openings are often studio logos and black frames, which
	# flatter every encoder equally and say nothing about the feature.
	start=$(awk -v t="$total" -v l="$clip_len" 'BEGIN {
		s = t * 0.2; if (s + l > t) s = t - l; if (s < 0) s = 0; printf "%.2f", s }')
	clip="$work/clip$n.mkv"
	ffmpeg -nostdin -hide_banner -loglevel error -y -ss "$start" -i "$src" \
		-t "$clip_len" -map 0:v:0 -c copy "$clip"
	clip_dur=$(duration_of "$clip")
	clip_size=$(size_of "$clip")
	src_codec=$(ffprobe -v error -select_streams v:0 -show_entries stream=codec_name,width,height \
		-of csv=p=0 "$clip" | awk -F, '{ print $1 " " $2 "x" $3 }')
	echo "[$n/$#] $(basename "$src") — $src_codec, ${clip_dur%.*}s clip from ${start%.*}s"

	for p in $profiles; do
		codec=${p%%:*}
		rest=${p#*:}
		crf=${rest%%:*}
		preset=${rest#*:}
		enc=$(encoder_for "$codec")
		out="$work/clip$n.$codec.$crf.$preset.mkv"
		# -benchmark prints the encode's wall time as rtime; extra stays unquoted
		# on purpose, split into words like the worker's strings.Fields.
		# shellcheck disable=SC2086
		rtime=$(ffmpeg -nostdin -hide_banner -benchmark -y -i "$clip" -map 0:v:0 \
			-c:v "$enc" -crf "$crf" -preset "$preset" $extra "$out" 2>&1 |
			sed -n 's/.*rtime=\([0-9.]*\)s.*/\1/p' | tail -n 1)
		if [ -z "$rtime" ] || [ ! -s "$out" ]; then
			echo "  $p: encode failed" >&2
			continue
		fi
		out_size=$(size_of "$out")
		q=$(quality_of "$out" "$clip")
		printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$p" "$clip_dur" "$rtime" "$clip_size" "$out_size" "$q" >>"$results"
		awk -v p="$p" -v d="$clip_dur" -v r="$rtime" -v cs="$clip_size" -v os="$out_size" -v q="$q" -v m="$metric" \
			'BEGIN { printf "  %-18s %7.1fs  %5.2fx realtime  %5.1f%% of source  %s %s\n", p, r, d / r, 100 * os / cs, m, q }'
	done
done

[ -s "$results" ] || { echo "no successful encodes" >&2; exit 1; }

echo
echo "Totals across all clips (speed and size are duration/byte-weighted; quality is the mean):"
printf '  %-18s %9s %10s %12s %8s %14s\n' profile "time" speed "size/source" "$metric" "time/hr video"
for p in $profiles; do
	awk -F '\t' -v p="$p" '$1 == p {
		d += $2; r += $3; cs += $4; os += $5; q += $6; k++
	} END {
		if (k == 0) exit
		printf "  %-18s %8.1fs %9.2fx %11.1f%% %8.2f %11.0f min\n", p, r, d / r, 100 * os / cs, q / k, 60 * r / d
	}' "$results"
done
echo
echo "\"time/hr video\" is how long one hour of footage like this would take on this machine."
[ "$metric" = SSIM ] && echo "This ffmpeg has no libvmaf; SSIM above 98 is generally transparent, below 96 visibly soft."
exit 0
