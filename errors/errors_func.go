package errors

// -----------------------------------------------------------------------------
// Functions
// -----------------------------------------------------------------------------

func IsOfKind(err error, kind Kind) bool {
	depth := 0
	for err != nil && depth < maxChainDepth {
		if ae, ok := err.(interface{ IsKind(Kind) bool }); ok {
			if ae.IsKind(kind) {
				return true
			}
		}
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			break
		}
		depth++
	}
	return false
}

func IsOfKinds(err error, kinds ...Kind) bool {
	depth := 0
	for err != nil && depth < maxChainDepth {
		if ae, ok := err.(interface{ IsKind(Kind) bool }); ok {
			for _, k := range kinds {
				if ae.IsKind(k) {
					return true
				}
			}
		}
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			break
		}
		depth++
	}
	return false
}

func IsOfClass(err error, class string) bool {
	depth := 0
	for err != nil && depth < maxChainDepth {
		if ae, ok := err.(interface{ IsClass(string) bool }); ok {
			if ae.IsClass(class) {
				return true
			}
		}
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			break
		}
		depth++
	}
	return false
}

func IsOfClasses(err error, classes ...string) bool {
	depth := 0
	for err != nil && depth < maxChainDepth {
		if ae, ok := err.(interface{ IsClass(string) bool }); ok {
			for _, c := range classes {
				if ae.IsClass(c) {
					return true
				}
			}
		}
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			break
		}
		depth++
	}
	return false
}
