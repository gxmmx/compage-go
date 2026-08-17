// Package configpflag adapts pflag flag sets to config.FlagSource without making
// the core config package depend on pflag. Source reports only flags explicitly
// changed by the caller, allowing unchanged flags to yield to lower-priority
// configuration sources.
package configpflag
