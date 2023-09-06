package vcard

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// A Decoder parses cards.
type Decoder struct {
	r *bufio.Reader
}

// NewDecoder creates a new Decoder reading cards from an io.Reader.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

func (dec *Decoder) readLine() (string, error) {
	l, err := dec.r.ReadString('\n')
	l = strings.TrimRight(l, "\r\n")
	if len(l) > 0 && err == io.EOF {
		return l, nil
	} else if err != nil {
		return l, err
	}

	for {
		next, err := dec.r.Peek(1)
		if err == io.EOF {
			break
		} else if err != nil {
			return l, err
		}

		if ch := next[0]; ch != ' ' && ch != '\t' {
			break
		}

		if _, err := dec.r.Discard(1); err != nil {
			return l, err
		}

		folded, err := dec.r.ReadString('\n')
		if err != nil {
			return l, err
		}
		l += strings.TrimRight(folded, "\r\n")
	}

	return l, nil
}

// Decode parses a single card.
func (dec *Decoder) Decode() (Card, error) {
	card := make(Card)

	var hasBegin, hasEnd bool
	for {
		l, err := dec.readLine()
		if err == io.EOF {
			break
		} else if err != nil {
			return card, err
		}

		k, f, err := parseLine(l)
		if err != nil {
			continue
		}

		if !hasBegin {
			if k == "BEGIN" {
				if strings.ToUpper(f.Value) != "VCARD" {
					return card, errors.New("vcard: invalid BEGIN value")
				}
				hasBegin = true
				continue
			} else {
				return card, errors.New("vcard: no BEGIN field found")
			}
		} else if k == "END" {
			if strings.ToUpper(f.Value) != "VCARD" {
				return card, errors.New("vcard: invalid END value")
			}
			hasEnd = true
			break
		}

		card[k] = append(card[k], f)
	}

	if !hasEnd {
		if !hasBegin {
			return nil, io.EOF
		}
		return card, errors.New("vcard: no END field found")
	}
	return card, nil
}

func parseLine(l string) (key string, field *Field, err error) {
	field = new(Field)
	field.Group, l = parseGroup(l)
	key, hasParams, l, err := parseKey(l)
	if err != nil {
		return
	}

	if hasParams {
		field.Params, l, err = parseParams(l)
		if err != nil {
			return
		}
	}

	field.Value = parseValue(l)
	return
}

func parseGroup(s string) (group, tail string) {
	i := strings.IndexAny(s, ".;:")
	if i < 0 || s[i] != '.' {
		return "", s
	}
	return s[:i], s[i+1:]
}

func parseKey(s string) (key string, params bool, tail string, err error) {
	i := strings.IndexAny(s, ";:")
	if i < 0 {
		err = errors.New("vcard: invalid property key")
		return
	}
	return strings.ToUpper(s[:i]), s[i] == ';', s[i+1:], nil
}

func parseParams(s string) (params Params, tail string, err error) {
	tail = s
	params = make(Params)
	for tail != "" {
		i := strings.IndexAny(tail, "=;:")
		if i < 0 {
			err = errors.New("vcard: malformed parameters")
			return
		}
		if tail[i] == ';' {
			tail = tail[i+1:]
			continue
		}

		k := strings.ToUpper(tail[:i])

		var values []string
		var more bool
		values, more, tail, err = parseParamValues(tail[i+1:])
		if err != nil {
			return
		}

		params[k] = append(params[k], values...)

		if !more {
			break
		}
	}
	return
}

func parseParamValues(s string) (values []string, more bool, tail string, err error) {
	if s == "" {
		return
	}
	quote := s[0]

	var vs string
	if quote == '"' {
		vs, tail, err = parseQuoted(s[1:], quote)
		if tail == "" || (tail[0] != ';' && tail[0] != ':') {
			err = errors.New("vcard: malformed quoted parameter value")
			return
		}
		more = tail[0] != ':'
		tail = tail[1:]
	} else {
		i := strings.IndexAny(s, ";:")
		if i < 0 {
			vs = s
		} else {
			vs, more, tail = s[:i], s[i] != ':', s[i+1:]
		}
	}

	values = strings.Split(vs, ",")
	for i, value := range values {
		values[i] = parseValue(value)
	}
	return
}

func parseQuoted(s string, quote byte) (value, tail string, err error) {
	tail = s
	var buf []rune
	for tail != "" {
		if tail[0] == quote {
			tail = tail[1:]
			break
		}

		var r rune
		r, _, tail, err = strconv.UnquoteChar(tail, quote)
		if err != nil {
			return
		}
		buf = append(buf, r)
	}
	value = string(buf)
	return
}

var valueParser = strings.NewReplacer("\\\\", "\\", "\\n", "\n", "\\,", ",")

func parseValue(s string) string {
	return valueParser.Replace(s)
}
