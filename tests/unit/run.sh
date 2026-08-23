#!/usr/bin/env bash
set -euo pipefail

test_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$test_dir/../.." && pwd)"

race=false
fuzz=false
coverage=false
target_dir="."

while (($# > 0)); do
	case "$1" in
	--all)
		target_dir="."
		;;
	--race)
		race=true
		;;
	--fuzz)
		fuzz=true
		;;
	--coverage)
		coverage=true
		;;
	-h|--help)
		echo "usage: $0 [--all] [--race | --fuzz | --coverage] [directory]"
		exit 0
		;;
	-*)
		echo "unit: unknown option: $1" >&2
		exit 2
		;;
	*)
		if [[ "$target_dir" != "." ]]; then
			echo "unit: only one directory may be specified" >&2
			exit 2
		fi
		target_dir="$1"
		;;
	esac
	shift
done

cd -- "$repo_root"

if [[ "$target_dir" == "all" ]]; then
	target_dir="."
fi

if [[ "$target_dir" != "." && ! -d "$target_dir" ]]; then
	echo "unit: directory does not exist: $target_dir" >&2
	exit 2
fi

if [[ "$fuzz" == true ]]; then
	if [[ "$race" == true || "$coverage" == true || "$target_dir" != "." ]]; then
		echo "unit: --fuzz cannot be combined with race, coverage, or a directory" >&2
		exit 2
	fi
	go test ./config -run '^$' -fuzz=FuzzJSONFormat -fuzztime=1s
	go test ./config -run '^$' -fuzz=FuzzStringConversion -fuzztime=1s
	exit 0
fi

if [[ "$coverage" == true ]]; then
	if [[ "$race" == true || "$target_dir" == "." ]]; then
		echo "unit: --coverage requires a package directory and cannot use --race" >&2
		exit 2
	fi
	coverage_name="$(basename -- "$target_dir")"
	coverage_file=".dist/coverage/${coverage_name}.out"
	mkdir -p .dist/coverage
	go test -count=1 -coverprofile="$coverage_file" "$repo_root/$target_dir"
	go tool cover -func="$coverage_file"
	exit 0
fi

if [[ "$target_dir" == "." ]]; then
	package_target="./..."
else
	package_target="$repo_root/$target_dir"
fi

if [[ "$race" == true ]]; then
	go test -race -count=1 "$package_target"
else
	go test -count=1 "$package_target"
fi
