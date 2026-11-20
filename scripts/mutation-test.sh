#!/usr/bin/env bash
# Changes one digit in each quote that has a number, one quote at a time,
# and checks that `cite check` reports that quote as missing. Pages come
# from the cache only, so run `cite check -cache DIR` first.
#   scripts/mutation-test.sh .cache
set -euo pipefail
cache=${1:-.cache}
go build -o /tmp/cite-mut ./cmd/cite
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
total=0 caught=0 skipped=0 missed=0
for f in book/*.md; do
  { grep -n '^  > .*[0-9]' "$f" || true; } | cut -d: -f1 | while read -r n; do echo "$f $n"; done
done > "$work/targets"
while read -r f n; do
  rm -rf "$work/book"; cp -r book "$work/book"
  # bump the first digit on that line: 0->1 ... 9->0
  awk -v n="$n" 'NR==n{ if (match($0,/[0-9]/)) { d=substr($0,RSTART,1); $0=substr($0,1,RSTART-1) ((d+1)%10) substr($0,RSTART+1) } } {print}' \
    "$f" > "$work/book/$(basename "$f")"
  q=$(sed -n "${n}p" "$work/book/$(basename "$f")" | sed 's/^  > //')
  /tmp/cite-mut check -book "$work/book" -cache "$cache" -offline -json "$work/r.json" >/dev/null 2>&1 || true
  st=$(jq -r --arg q "$q" '[.results[] | select(.quote == $q) | .status] | first // "none"' "$work/r.json")
  total=$((total+1))
  case $st in
    missing) caught=$((caught+1)) ;;
    unreachable) skipped=$((skipped+1)) ;;
    *) missed=$((missed+1)); echo "not caught: $f:$n ($st)" ;;
  esac
done < "$work/targets"
echo "changed one digit in $total quotes: $caught caught, $missed missed, $skipped skipped (page not cached)"
[ "$missed" -eq 0 ]
