package errors

// -----------------------------------------------------------------------------
// Functions
// -----------------------------------------------------------------------------

func IsAppError(err error) bool {
	depth := 0
	for err != nil && depth < maxChainDepth {
		if _, ok := err.(ApplicationError); ok {
			return true
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

func IsOfKindClass(err error, kind Kind, class string) bool {
	depth := 0
	for err != nil && depth < maxChainDepth {
		if ae, ok := err.(interface{ IsKind(Kind) bool }); ok {
			if ae.IsKind(kind) {
				if ace, ok := err.(interface{ IsClass(string) bool }); ok {
					return ace.IsClass(class)
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
