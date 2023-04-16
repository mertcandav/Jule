package lit

import (
	"strconv"
	"unicode/utf8"
)

// Reports whether kind is byte literal and returns
// literal without quotes.
//
// Byte literal patterns:
//  - 'x': 0 <= x && x <= 255
//  - '\xhh'
//  - '\nnn'
func Is_byte_lit(kind string) (string, bool) {
	if len(kind) < 3 {
		return "", false
	}

	kind = kind[1 : len(kind)-1] // Remove quotes.
	is_byte := false
	
	// TODO: Add support for byte escape sequences.
	switch {
	case len(kind) == 1 && kind[0] <= 255:
		is_byte = true

	case kind[0] == '\\' && kind[1] == 'x':
		is_byte = true

	case kind[0] == '\\' && kind[1] >= '0' && kind[1] <= '7':
		is_byte = true
	}

	return kind, is_byte
}

// Returns rune value string from bytes.
// Bytes are represents rune literal, allows escape sequences.
// Returns empty string if len(bytes) == 0
func To_rune(bytes []byte) string {
	if len(bytes) == 0 {
		return ""
	}

	var r rune = 0
	if bytes[0] == '\\' && len(bytes) > 1 {
		i := 0
		r = rune_from_esq_seq(bytes, &i)
	} else {
		r, _ = utf8.DecodeRune(bytes)
	}

	return Rtoa(r)
}

// Returns rune as rune value string.
func Rtoa(r rune) string { return "0x" + strconv.FormatInt(int64(r), 16) }

func try_btoa_common_esq(bytes []byte) (seq byte, ok bool) {
	if len(bytes) < 2 || bytes[0] != '\\' {
		return
	}

	switch bytes[1] {
	case '\\':
		seq = '\\'

	case '\'':
		seq = '\''

	case '"':
		seq = '"'

	case 'a':
		seq = '\a'

	case 'b':
		seq = '\b'

	case 'f':
		seq = '\f'

	case 'n':
		seq = '\n'

	case 'r':
		seq = '\r'

	case 't':
		seq = '\t'

	case 'v':
		seq = '\v'
	}

	ok = seq != 0
	return
}

func rune_from_esq_seq(bytes []byte, i *int) rune {
	b, ok := try_btoa_common_esq(bytes[*i:])
	*i++
	if ok {
		return rune(b)
	}

	switch bytes[*i] {
	case 'u':
		rc, _ := strconv.ParseUint(string(bytes[*i+1:*i+5]), 16, 32)
		*i += 4
		r := rune(rc)
		return r

	case 'U':
		rc, _ := strconv.ParseUint(string(bytes[*i+1:*i+9]), 16, 32)
		*i += 8
		r := rune(rc)
		return r

	case 'x':
		seq := bytes[*i : *i+3]
		*i += 2
		b, _ := strconv.ParseUint(string(seq), 16, 8)
		return rune(b)

	default:
		seq := bytes[*i : *i+3]
		*i += 2
		b, _ := strconv.ParseUint(string(seq), 8, 8)
		return rune(b)
	}
}
