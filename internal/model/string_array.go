package model

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// StringArray maps a Postgres text[] column. It is written as an array
// literal ({"a","b"}) and read back from the text form the pgx stdlib driver
// returns, so it needs no extra driver dependency. A nil slice is stored as
// an empty array ('{}'), never NULL.
type StringArray []string

// Value implements driver.Valuer.
func (a StringArray) Value() (driver.Value, error) {
	var b strings.Builder
	b.WriteByte('{')
	for i, s := range a {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		for _, r := range s {
			if r == '"' || r == '\\' {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String(), nil
}

// Scan implements sql.Scanner for the text representation of a
// one-dimensional text[] ('{a,"b c",NULL}'). NULL elements are dropped.
func (a *StringArray) Scan(src any) error {
	var text string
	switch v := src.(type) {
	case nil:
		*a = StringArray{}
		return nil
	case []byte:
		text = string(v)
	case string:
		text = v
	case []string:
		*a = append(StringArray{}, v...)
		return nil
	default:
		return fmt.Errorf("StringArray: unsupported source type %T", src)
	}
	parsed, err := parseTextArray(text)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

func parseTextArray(text string) (StringArray, error) {
	if len(text) < 2 || text[0] != '{' || text[len(text)-1] != '}' {
		return nil, fmt.Errorf("StringArray: invalid array literal %q", text)
	}
	body := []rune(text[1 : len(text)-1])
	out := StringArray{}
	for i := 0; i < len(body); {
		var elem strings.Builder
		quoted := body[i] == '"'
		if quoted {
			i++
			for ; i < len(body) && body[i] != '"'; i++ {
				if body[i] == '\\' && i+1 < len(body) {
					i++
				}
				elem.WriteRune(body[i])
			}
			if i >= len(body) {
				return nil, fmt.Errorf("StringArray: unterminated quote in %q", text)
			}
			i++ // closing quote
		} else {
			for ; i < len(body) && body[i] != ','; i++ {
				elem.WriteRune(body[i])
			}
		}
		if s := elem.String(); quoted || s != "NULL" {
			out = append(out, s)
		}
		if i < len(body) {
			if body[i] != ',' {
				return nil, fmt.Errorf("StringArray: unexpected %q in %q", body[i], text)
			}
			i++
		}
	}
	return out, nil
}
