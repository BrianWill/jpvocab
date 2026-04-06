#!/usr/bin/env bash
# Print sorted line counts for all .go and .js files under ,
# then totals per language.

shopt -s nullglob

DIR="$(cd "$(dirname "$0")/src" && pwd)"

go_src_files=()
go_test_files=()
for f in "$DIR"/*.go; do
    [[ "$f" == *_test.go ]] && go_test_files+=("$f") || go_src_files+=("$f")
done
js_src_files=("$DIR"/static/*.js)
js_test_files=("$DIR"/static/tests/*.js)
html_files=("$DIR"/static/*.html "$DIR"/templates/*.html)
css_files=("$DIR"/static/*.css)

all_files=("${go_src_files[@]}" "${go_test_files[@]}" "${js_src_files[@]}" "${js_test_files[@]}")

echo "Lines  File"
echo "-----  ----"
for f in "${all_files[@]}"; do
    lines=$(wc -l < "$f")
    rel="${f#"$DIR"/}"
    printf "%5d  %s\n" "$lines" "$rel"
done | sort -rn

echo ""

go_src_total=0
for f in "${go_src_files[@]}"; do
    go_src_total=$((go_src_total + $(wc -l < "$f")))
done

go_test_total=0
for f in "${go_test_files[@]}"; do
    go_test_total=$((go_test_total + $(wc -l < "$f")))
done

js_src_total=0
for f in "${js_src_files[@]}"; do
    js_src_total=$((js_src_total + $(wc -l < "$f")))
done

js_test_total=0
for f in "${js_test_files[@]}"; do
    js_test_total=$((js_test_total + $(wc -l < "$f")))
done

html_total=0
for f in "${html_files[@]}"; do
    html_total=$((html_total + $(wc -l < "$f")))
done

css_total=0
for f in "${css_files[@]}"; do
    css_total=$((css_total + $(wc -l < "$f")))
done

printf "Go total:  %d (src) + %d (test)\n" "$go_src_total" "$go_test_total"
printf "JS total:  %d (src) + %d (test)\n" "$js_src_total" "$js_test_total"
printf "HTML total:      %d\n" "$html_total"
printf "CSS total:       %d\n" "$css_total"
