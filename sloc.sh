#!/usr/bin/env bash
# Print sorted line counts for all .go and .js files under prototype/backend,
# then totals per language.

DIR="$(cd "$(dirname "$0")/prototype/backend" && pwd)"

go_files=("$DIR"/*.go)
js_files=("$DIR"/static/*.js "$DIR"/static/tests/*.js)
html_files=("$DIR"/static/*.html "$DIR"/templates/*.html)
css_files=("$DIR"/static/*.css)

all_files=("${go_files[@]}" "${js_files[@]}")

echo "Lines  File"
echo "-----  ----"
for f in "${all_files[@]}"; do
    lines=$(wc -l < "$f")
    rel="${f#"$DIR"/}"
    printf "%5d  %s\n" "$lines" "$rel"
done | sort -rn

echo ""

go_total=0
for f in "${go_files[@]}"; do
    go_total=$((go_total + $(wc -l < "$f")))
done

js_total=0
for f in "${js_files[@]}"; do
    js_total=$((js_total + $(wc -l < "$f")))
done

html_total=0
for f in "${html_files[@]}"; do
    html_total=$((html_total + $(wc -l < "$f")))
done

css_total=0
for f in "${css_files[@]}"; do
    css_total=$((css_total + $(wc -l < "$f")))
done

printf "Go total:   %d\n" "$go_total"
printf "JS total:   %d\n" "$js_total"
printf "HTML total: %d\n" "$html_total"
printf "CSS total:  %d\n" "$css_total"
