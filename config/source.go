package config

// Source identifies a configuration layer.
type Source uint8

const (
	SourceNone Source = iota
	SourceDefault
	SourceFile
	SourceEnv
	SourceFlag
	SourceSet
)

func (s Source) String() string {
	switch s {
	case SourceNone:
		return "none"
	case SourceDefault:
		return "default"
	case SourceFile:
		return "file"
	case SourceEnv:
		return "env"
	case SourceFlag:
		return "flag"
	case SourceSet:
		return "set"
	default:
		return "none"
	}
}

// Origin describes the winning source and its source-specific detail.
type Origin struct {
	Source Source
	Detail string
}

type state[T any] struct {
	values     T
	origins    map[string]Origin
	present    map[Source]map[string]bool
	layers     map[Source]map[string]any
	path       string
	fileLoaded bool
}

func cloneRaw(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = cloneRawValue(v)
	}
	return out
}

func cloneRawValue(value any) any {
	if values, ok := value.([]string); ok {
		return append([]string(nil), values...)
	}
	return value
}

func cloneLayers(in map[Source]map[string]any) map[Source]map[string]any {
	out := map[Source]map[string]any{}
	for _, source := range []Source{SourceDefault, SourceFile, SourceEnv, SourceFlag, SourceSet} {
		out[source] = cloneRaw(in[source])
	}
	return out
}
