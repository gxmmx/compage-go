#!/usr/bin/env bash
set -euo pipefail

check_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$check_dir/../.." && pwd)"
unit_runner="$repo_root/.tests/unit/run.sh"
coverage_report="$repo_root/.tests/unit/coverage.sh"

full=false
target_dir="."

while (($# > 0)); do
	case "$1" in
	--all)
		full=true
		;;
	-h|--help)
		echo "usage: $0 [--all] [directory]"
		exit 0
		;;
	-*)
		echo "check: unknown option: $1" >&2
		exit 2
		;;
	*)
		if [[ "$target_dir" != "." ]]; then
			echo "check: only one directory may be specified" >&2
			exit 2
		fi
		target_dir="$1"
		;;
	esac
	shift
done

cd -- "$repo_root"

# Keep the individual Mise tasks as the source of truth for repository-wide checks.
mise run fmt:check
mise run mod:check
mise run lint

if [[ "$full" == true ]]; then
	# Race detection and coverage instrument the same test execution.
	"$unit_runner" --race --coverage "$target_dir"
	"$coverage_report" "$target_dir"
	"$unit_runner" --fuzz "$target_dir"
else
	"$unit_runner" "$target_dir"
fi
