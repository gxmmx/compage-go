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
	if [[ "$race" == true || "$coverage" == true ]]; then
		echo "unit: --fuzz cannot be combined with race or coverage" >&2
		exit 2
	fi

	if [[ "$target_dir" == "." ]]; then
		package_list="$(go list ./...)"
	else
		package_list="$(go list "$repo_root/$target_dir")"
	fi

	fuzz_ran=false
	while IFS= read -r package_path; do
		fuzz_targets="$(go test -vet=off -list '^Fuzz' "$package_path" | awk '/^Fuzz[A-Za-z0-9_]*$/ {print}')"
		while IFS= read -r fuzz_target; do
			[[ -z "$fuzz_target" ]] && continue
			fuzz_ran=true
			echo "fuzz: $package_path/$fuzz_target"
			go test -vet=off "$package_path" -run '^$' -fuzz="^${fuzz_target}$" -fuzztime=1s
		done <<< "$fuzz_targets"
	done <<< "$package_list"

	if [[ "$fuzz_ran" == false ]]; then
		echo "fuzz: no fuzz targets found for $target_dir"
	fi
	exit 0
fi

if [[ "$coverage" == true ]]; then
	coverage_dir=".dist/coverage"
	mkdir -p "$coverage_dir"
	find "$coverage_dir" -maxdepth 1 -type f -name '*.out' -delete

	if [[ "$target_dir" == "." ]]; then
		package_list="$(go list -f '{{.ImportPath}}|{{.Dir}}' ./...)"
		while IFS='|' read -r package_path package_dir; do
			package_name="${package_dir#"$repo_root"/}"
			if [[ "$package_name" == "$package_dir" ]]; then
				package_name="root"
			fi
			package_name="${package_name//\//_}"
			coverage_file="$coverage_dir/${package_name}.out"
			echo "coverage: $package_path"
			test_flags=(-count=1 -vet=off)
			if [[ "$race" == true ]]; then
				test_flags+=(-race)
			fi
			go test "${test_flags[@]}" -coverprofile="$coverage_file" "$package_path"
		done <<< "$package_list"
	else
		coverage_name="${target_dir#./}"
		coverage_name="${coverage_name//\//_}"
		coverage_file="$coverage_dir/${coverage_name}.out"
		test_flags=(-count=1 -vet=off)
		if [[ "$race" == true ]]; then
			test_flags+=(-race)
		fi
		go test "${test_flags[@]}" -coverprofile="$coverage_file" "$repo_root/$target_dir"
	fi
	exit 0
fi

if [[ "$target_dir" == "." ]]; then
	package_target="./..."
else
	package_target="$repo_root/$target_dir"
fi

test_flags=(-count=1 -vet=off)
if [[ "$race" == true ]]; then
	test_flags+=(-race)
fi
go test "${test_flags[@]}" "$package_target"
