#!/usr/bin/env bash
set -euo pipefail

test_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$test_dir/../.." && pwd)"
coverage_dir="$repo_root/.dist/coverage"
target_dir="${1:-.}"

if (($# > 1)); then
	echo "coverage: only one package directory may be specified" >&2
	exit 2
fi

if [[ "$target_dir" == "all" ]]; then
	target_dir="."
fi

if [[ ! -d "$coverage_dir" ]]; then
	echo "coverage: no reports found; run mise run test:coverage first" >&2
	exit 1
fi

if [[ "$target_dir" == "." ]]; then
	profiles="$(find "$coverage_dir" -maxdepth 1 -type f -name '*.out' -print | sort)"
else
	profile_name="${target_dir#./}"
	profile_name="${profile_name//\//_}"
	profiles="$coverage_dir/${profile_name}.out"
	if [[ ! -f "$profiles" ]]; then
		echo "coverage: report not found for $target_dir; run mise run test:coverage -- $target_dir first" >&2
		exit 1
	fi
fi
if [[ -z "$profiles" ]]; then
	echo "coverage: no reports found; run mise run test:coverage first" >&2
	exit 1
fi

printf '%-28s %10s  %s\n' "Package" "Coverage" "Profile"
printf '%-28s %10s  %s\n' "-------" "--------" "-------"

total_statements=0
covered_statements=0
package_count=0

while IFS= read -r profile; do
	[[ -z "$profile" ]] && continue

	profile_name="${profile##*/}"
	package_name="${profile_name%.out}"
	package_name="${package_name//_//}"
	coverage="$(go tool cover -func="$profile" | awk '$1 == "total:" {print $3}')"
	coverage="${coverage%%%}"
	read -r statements covered < <(
		awk '
			NR == 1 && $1 == "mode:" { next }
			{ statements += $2; if ($3 > 0) covered += $2 }
			END { print statements + 0, covered + 0 }
		' "$profile"
	)

	printf '%-28s %9s%%  %s\n' "$package_name" "$coverage" "$profile_name"
	total_statements=$((total_statements + statements))
	covered_statements=$((covered_statements + covered))
	package_count=$((package_count + 1))
done <<< "$profiles"

if ((total_statements > 0)); then
	total_coverage="$(awk -v covered="$covered_statements" -v statements="$total_statements" 'BEGIN { printf "%.1f", 100 * covered / statements }')"
else
	total_coverage="0.0"
fi

printf '%s\n' ""
printf '%-28s %9s%%  %d package(s), %d/%d statements\n' \
	"Total" "$total_coverage" "$package_count" "$covered_statements" "$total_statements"
