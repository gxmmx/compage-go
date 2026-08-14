package errx

import "reflect"

// KindOf returns the effective semantic classification of err. It returns
// false for nil, unclassified, invalid, and ambiguously classified joined
// errors. The nearest valid non-Unknown classification wins.
func KindOf(err error) (Kind, bool) {
	return kindOf(err, make(map[visit]struct{}))
}

func kindOf(err error, seen map[visit]struct{}) (Kind, bool) {
	if isNilError(err) || markSeen(err, seen) {
		return Unknown, false
	}

	if classified, ok := err.(Classified); ok {
		kind := classified.Kind()
		if kind.Valid() && kind != Unknown {
			return kind, true
		}
	}

	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var kind Kind
		found := false
		for _, child := range joined.Unwrap() {
			childKind, ok := kindOf(child, seen)
			if !ok {
				continue
			}
			if !found {
				kind, found = childKind, true
				continue
			}
			if childKind != kind {
				return Unknown, false
			}
		}
		return kind, found
	}

	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return kindOf(wrapped.Unwrap(), seen)
	}
	return Unknown, false
}

// KindsOf returns every distinct valid non-Unknown kind in err's unwrap tree.
// Kinds appear in deterministic depth-first, outer-to-inner order.
func KindsOf(err error) []Kind {
	seen := make(map[visit]struct{})
	found := make(map[Kind]struct{})
	var kinds []Kind

	var walk func(error)
	walk = func(current error) {
		if isNilError(current) || markSeen(current, seen) {
			return
		}
		if classified, ok := current.(Classified); ok {
			kind := classified.Kind()
			if kind.Valid() && kind != Unknown {
				if _, exists := found[kind]; !exists {
					found[kind] = struct{}{}
					kinds = append(kinds, kind)
				}
			}
		}
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				walk(child)
			}
			return
		}
		if wrapped, ok := current.(interface{ Unwrap() error }); ok {
			walk(wrapped.Unwrap())
		}
	}

	walk(err)
	return kinds
}

// IsKind reports whether want is err's effective kind. It does not search for
// matching classifications hidden by an outer reclassification or a conflict.
func IsKind(err error, want Kind) bool {
	kind, ok := KindOf(err)
	return ok && kind == want
}

type visit struct {
	typ reflect.Type
	ptr uintptr
}

// markSeen reports whether err was visited before. Error cycles require a
// reference value; ordinary value errors cannot form a recursive unwrap tree.
func markSeen(err error, seen map[visit]struct{}) bool {
	v := reflect.ValueOf(err)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return false
	}
	key := visit{typ: v.Type(), ptr: v.Pointer()}
	if _, ok := seen[key]; ok {
		return true
	}
	seen[key] = struct{}{}
	return false
}

func isNilError(err error) bool {
	if err == nil {
		return true
	}
	v := reflect.ValueOf(err)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
