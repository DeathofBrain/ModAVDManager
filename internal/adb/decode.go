package adb

import (
	"strings"
	"unicode/utf8"
)

const (
	cpUTF8    = 65001
	cpGBK     = 936
	cpGB18030 = 54936
)

func DecodeOutput(b []byte) string { return decodeOutput(b) }
func decodeOutput(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if utf8.Valid(b) {
		return string(b)
	}
	for _, cp := range []uint32{ansiCodePage(), cpGBK, cpGB18030} {
		if s, ok := decodeCodePage(cp, b); ok {
			return s
		}
	}
	return latin1(b)
}
func latin1(b []byte) string {
	var s strings.Builder
	s.Grow(len(b) * 2)
	for _, c := range b {
		s.WriteRune(rune(c))
	}
	return s.String()
}
func utf8Replace(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return string([]rune(s))
}
