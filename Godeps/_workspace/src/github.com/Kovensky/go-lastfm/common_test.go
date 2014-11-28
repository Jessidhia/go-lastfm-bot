package lastfm_test

import (
	"fmt"
	"testing"
)

func Expect(T *testing.T, item string, expected, actual interface{}) bool {
	if expected != actual {
		switch expected.(type) {
		case string, fmt.Stringer:
			T.Errorf("Expected %v %q -- Got %q ", item, expected, actual)
		default:
			T.Errorf("Expected %v %v -- Got %v ", item, expected, actual)
		}
		return false
	}
	return true
}
