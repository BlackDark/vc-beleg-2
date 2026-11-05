package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Limits of the free-form extra object of a data entry.
const (
	maxExtraDepth      = 5   // nesting levels, extra itself is level 1
	maxExtraKeyRunes   = 100 // the report prints keys in a narrow column
	maxExtraValueRunes = 500 // per string value
	maxExponent        = 400 // before big-number parsing, to bound memory
	entryDepthLimit    = maxExtraDepth + 1
)

// walkError aborts the parse of one entry (structure problems that make
// the entry ambiguous, not just invalid).
type walkError struct {
	code, field, msg string
	params           []string
}

func (w *walkError) Error() string { return w.msg }

// readEntry decodes one JSON object. Unlike json.Unmarshal it rejects
// duplicate keys (the last one would silently win and the same entry could
// hash two ways), limits the nesting depth and NFC-normalizes all strings,
// so visually identical entries are byte-identical.
func readEntry(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	v, err := readValue(dec, 0)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not an object")
	}
	return m, nil
}

func readValue(dec *json.Decoder, depth int) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		if depth+1 > entryDepthLimit {
			return nil, &walkError{"data.extra_too_deep", "extra", "extra is nested too deeply", []string{"limit", fmt.Sprint(maxExtraDepth)}}
		}
		if t == '{' {
			m := map[string]any{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k := norm.NFC.String(kt.(string))
				if _, dup := m[k]; dup {
					return nil, &walkError{"data.key_duplicate", "", "a key appears twice", []string{"key", clip(k)}}
				}
				if m[k], err = readValue(dec, depth+1); err != nil {
					return nil, err
				}
			}
			_, err := dec.Token() // }
			return m, err
		}
		list := []any{}
		for dec.More() {
			v, err := readValue(dec, depth+1)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		_, err := dec.Token() // ]
		return list, err
	case string:
		return norm.NFC.String(t), nil
	default:
		return tok, nil // json.Number, bool, nil
	}
}

func clip(s string) string {
	if utf8.RuneCountInString(s) <= 40 {
		return s
	}
	return string([]rune(s)[:40]) + "…"
}

// hasHiddenRune reports control and format characters (e.g. U+202E, which
// reverses the text direction and can fake a name on an evidence page).
func hasHiddenRune(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) })
}

// cleanExtra checks the extra object and rewrites its numbers canonically.
// Only the first problem of each kind is reported.
func cleanExtra(i int, extra map[string]any) (map[string]any, []EntryError) {
	seen := map[string]bool{}
	var errs []EntryError
	add := func(e EntryError) {
		if !seen[e.Code] {
			seen[e.Code] = true
			errs = append(errs, e)
		}
	}
	var walk func(v any) any
	walk = func(v any) any {
		switch t := v.(type) {
		case map[string]any:
			for k, x := range t {
				if k == "" || utf8.RuneCountInString(k) > maxExtraKeyRunes || hasHiddenRune(k) {
					add(entryErr(i, "extra", "data.extra_key_invalid", "extra keys must be 1 to 100 characters without control or invisible formatting characters", "limit", fmt.Sprint(maxExtraKeyRunes)))
				}
				t[k] = walk(x)
			}
		case []any:
			for j, x := range t {
				t[j] = walk(x)
			}
		case string:
			if utf8.RuneCountInString(t) > maxExtraValueRunes {
				add(entryErr(i, "extra", "data.extra_value_too_long", "an extra value is too long", "limit", fmt.Sprint(maxExtraValueRunes)))
			}
		case json.Number:
			n, ok := canonicalNumber(string(t))
			if !ok {
				add(entryErr(i, "extra", "data.extra_number_invalid", "extra numbers must fit a 64-bit float"))
				return v
			}
			return n
		}
		return v
	}
	walk(extra)
	return extra, errs
}

// canonicalNumber writes a JSON number as plain decimal without trailing
// zeros, so 2, 2.0, 2e0 and 0.2e1 are the same value. It fails when the
// number does not fit a float64 (e.g. 1e999), which Typst could not read.
func canonicalNumber(s string) (json.Number, bool) {
	if f, err := strconv.ParseFloat(s, 64); err != nil || f > 1.7976931348623157e308 || f < -1.7976931348623157e308 {
		return "", false
	}
	if _, exp, ok := strings.Cut(strings.ToLower(s), "e"); ok {
		if n, err := strconv.Atoi(exp); err != nil || n > maxExponent || n < -maxExponent {
			return "", false
		}
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return "", false
	}
	out := r.FloatString(maxExponent + 20)
	if strings.Contains(out, ".") {
		out = strings.TrimRight(strings.TrimRight(out, "0"), ".")
	}
	if out == "-0" || out == "" {
		out = "0"
	}
	return json.Number(out), true
}
