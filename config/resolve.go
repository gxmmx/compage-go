package config

import (
	"os"
	"path/filepath"
	"strings"
)

type fileMode int

const (
	fileModeNone     fileMode = iota // no file tier
	fileModeExplicit                 // ConfigEnv → full path via SetConfigFile
	fileModePaths                    // name + type + paths via AddConfigPath
)

type fileSetup struct {
	mode     fileMode
	path     string // only for fileModeExplicit
	fileType string
}

func resolveFileSetup(opts *options, diag *diagBuffer) (fileSetup, error) {
	if opts.configEnv != "" {
		path := strings.TrimSpace(os.Getenv(opts.configEnv))
		if path != "" {
			ft := inferFileType(path, opts.fileType)
			diag.log("resolve: using ConfigEnv",
				"env_var", opts.configEnv,
				"path", path,
				"type", ft,
			)
			return fileSetup{mode: fileModeExplicit, path: path, fileType: ft}, nil
		}
		diag.log("resolve: ConfigEnv variable empty, falling back to paths",
			"env_var", opts.configEnv,
		)
	}

	if len(opts.paths) > 0 {
		diag.log("resolve: using path search",
			"name", opts.name,
			"type", opts.fileType,
			"paths", strings.Join(opts.paths, ", "),
		)
		return fileSetup{mode: fileModePaths, fileType: opts.fileType}, nil
	}

	diag.log("resolve: no file tier (no ConfigEnv, no paths)")
	return fileSetup{mode: fileModeNone}, nil
}

func storePath(opts *options) string {
	if opts.configEnv != "" {
		path := strings.TrimSpace(os.Getenv(opts.configEnv))
		if path != "" {
			return path
		}
	}

	if len(opts.paths) > 0 {
		return filepath.Join(opts.paths[0], opts.name+"."+opts.fileType)
	}

	return ""
}

func inferFileType(path string, fallback string) string {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	switch ext {
	case "toml", "yaml", "yml", "json":
		return ext
	default:
		if fallback != "" {
			return fallback
		}
		return "toml"
	}
}
