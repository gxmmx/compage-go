package config

// FlagSource supplies externally owned flags. Only changed flags participate.
type FlagSource interface {
	Lookup(name string) (value string, changed bool, found bool)
}
