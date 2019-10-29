package string_parser

import (
	"strconv"
	"unicode"
	"unicode/utf8"
)

//------------------------------------------------------------------------------

func SkipSpaces(s string) int {
	var i int
	var w int

	for i := 0; i < len(s); i += w {
		c, width := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError {
			return -1
		}
		if c != 9 && c != 32 {
			break
		}
		w = width
	}
	return i
}

func GetUint64(s string) (uint64, int) {
	var i int
	var w int

	for i = 0; i < len(s); i += w {
		c, width := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError {
			return 0, -1
		}
		if c < 48 || c > 57 {
			break
		}
		w = width
	}

	value, err := strconv.ParseUint(s[:i], 10, 64)
	if err != nil {
		return 0, -1
	}
	return value, i
}

func GetFloat64(s string) (float64, int) {
	var i int
	var w int

	dotSeen := false
	for i = 0; i < len(s); i += w {
		c, width := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError {
			return 0, -1
		}
		if c == 46 {
			if dotSeen {
				return 0, -1
			}
			dotSeen = true
		} else if c < 48 || c > 57 {
			break
		}
		w = width
	}

	value, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, -1
	}
	return value, i
}

func GetText(s string) (string, int) {
	var i int
	var w int

	for i = 0; i < len(s); i += w {
		c, width := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError {
			return "", -1
		}
		if !unicode.IsLetter(c) {
			break
		}
		w = width
	}
	return s[:i], i
}
