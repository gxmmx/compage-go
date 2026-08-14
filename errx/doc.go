// Package errx provides transport-neutral semantic classification for errors.
//
// Kinds describe how an error is interpreted at an abstraction boundary, while
// an error's unwrap tree preserves its causes. KindOf uses the nearest valid
// classification: an outer error may deliberately reclassify its cause. For a
// joined error without an outer classification, all classified branches must
// agree before it has one effective kind.
//
// Error is an immutable general-purpose wrapper. Domain packages should define
// their own contextual error types and use Classified to expose their kind.
// An Unwrap method is an API commitment: callers can use errors.Is and
// errors.As to observe the returned cause.
package errx
