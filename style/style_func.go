package style

import (
	"fmt"
	"strings"
)

// -----------------------------------------------------------------------------
// Functions
// -----------------------------------------------------------------------------

func genCode(a attr) string {
	return fmt.Sprintf("\033[%dm", a)
}

// Returns the attribute code for the given color
func colorFromString(c string) attr {
	if val, ok := attributeNames[strings.ToLower(c)]; ok {
		if val >= 30 && val <= 37 || val >= 90 && val <= 97 {
			return val
		}
	}
	return 0
}

// returns the attribute codes for the given style string
func attrsFromString(a string) []attr {
	attrs := []attr{}
	for _, attrStr := range strings.Split(a, ",") {
		if val, ok := attributeNames[strings.ToLower(attrStr)]; ok {
			attrs = append(attrs, val)
		}
	}
	return attrs
}
