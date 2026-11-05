package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/blackdark/vc-beleg-2/internal/money"
)

// Limits of a data file (JSON upload that creates receipts without a scan).
const (
	maxDataEntries    = 100
	maxDataEntryBytes = 16 << 10
	maxMerchantRunes  = 200
	maxReceiptCents   = 100000 // same ceiling as ReceiptInput.Total
)

// EntryError says why one entry of a data file was rejected. Index is the
// position in the file, starting at 0 (a single object is entry 0).
type EntryError struct {
	Index   int               `json:"index"`
	Field   string            `json:"field" doc:"date, total, merchant, extra, or empty for the entry as a whole"`
	Code    string            `json:"code" doc:"error code, for translation"`
	Message string            `json:"message"`
	Params  map[string]string `json:"params,omitempty"`
}

// DataEntry is a valid entry of a data file. Canonical is the normalized
// JSON record that is stored as the original and checksummed, so the same
// entry is found again whatever its key order, spacing or number notation.
type DataEntry struct {
	Date      string
	Total     money.Cents
	Merchant  string
	Canonical []byte
}

// canonicalRecord fixes the key order of the stored record; Extra keys are
// sorted by encoding/json.
type canonicalRecord struct {
	Date     string         `json:"date"`
	Merchant string         `json:"merchant"`
	Total    string         `json:"total"`
	Extra    map[string]any `json:"extra,omitempty"`
}

// ParseDataFile reads a JSON file holding one receipt object or an array of
// them. It is all-or-nothing: when any entry is invalid, no entries are
// returned, only the errors of every invalid entry.
func ParseDataFile(b []byte) ([]DataEntry, []EntryError, error) {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	dec := json.NewDecoder(bytes.NewReader(b))
	var top json.RawMessage
	if err := dec.Decode(&top); err != nil {
		return nil, nil, Invalid("data.json_invalid", "the file is not valid JSON")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, nil, Invalid("data.json_invalid", "the file is not valid JSON")
	}
	var raws []json.RawMessage
	switch {
	case bytes.HasPrefix(top, []byte("[")):
		if err := json.Unmarshal(top, &raws); err != nil {
			return nil, nil, Invalid("data.json_invalid", "the file is not valid JSON")
		}
	case bytes.HasPrefix(top, []byte("{")):
		raws = []json.RawMessage{top}
	default:
		return nil, nil, Invalid("data.shape", "the file must hold a receipt object or an array of them")
	}
	if len(raws) == 0 {
		return nil, nil, Invalid("data.empty", "the file holds no receipts")
	}
	if len(raws) > maxDataEntries {
		return nil, nil, Invalid("data.too_many", "the file holds too many receipts", "limit", fmt.Sprint(maxDataEntries))
	}
	var entries []DataEntry
	var errs []EntryError
	for i, raw := range raws {
		e, entryErrs := parseDataEntry(i, raw)
		errs = append(errs, entryErrs...)
		entries = append(entries, e)
	}
	if len(errs) > 0 {
		return nil, errs, nil
	}
	return entries, nil, nil
}

func entryErr(i int, field, code, msg string, params ...string) EntryError {
	e := EntryError{Index: i, Field: field, Code: code, Message: msg}
	if len(params) > 0 {
		e.Params = map[string]string{}
		for j := 0; j+1 < len(params); j += 2 {
			e.Params[params[j]] = params[j+1]
		}
	}
	return e
}

func parseDataEntry(i int, raw json.RawMessage) (DataEntry, []EntryError) {
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return DataEntry{}, []EntryError{entryErr(i, "", "data.entry_not_object", "the entry must be an object")}
	}
	fields, err := readEntry(raw)
	var we *walkError
	if errors.As(err, &we) {
		return DataEntry{}, []EntryError{entryErr(i, we.field, we.code, we.msg, we.params...)}
	}
	if err != nil {
		return DataEntry{}, []EntryError{entryErr(i, "", "data.entry_not_object", "the entry must be an object")}
	}
	var errs []EntryError
	for k := range fields {
		switch k {
		case "date", "total", "merchant", "extra":
		default:
			errs = append(errs, entryErr(i, k, "data.field_unknown", "unknown field; put additional data into extra", "field", k))
		}
	}
	var e DataEntry
	rec := canonicalRecord{}

	switch v := fields["date"].(type) {
	case nil:
		errs = append(errs, entryErr(i, "date", "data.field_missing", "date is required", "field", "date"))
	case string:
		if _, err := time.Parse(time.DateOnly, v); err != nil || len(v) != len(time.DateOnly) {
			errs = append(errs, entryErr(i, "date", "data.date_invalid", "the date must be YYYY-MM-DD"))
		} else {
			e.Date, rec.Date = v, v
		}
	default:
		errs = append(errs, entryErr(i, "date", "data.date_invalid", "the date must be YYYY-MM-DD"))
	}

	switch v := fields["merchant"].(type) {
	case nil:
		errs = append(errs, entryErr(i, "merchant", "data.field_missing", "merchant is required", "field", "merchant"))
	case string:
		name := norm.NFC.String(strings.TrimSpace(v))
		if hasHiddenRune(name) {
			errs = append(errs, entryErr(i, "merchant", "data.merchant_characters", "the merchant must not contain control or invisible formatting characters"))
		} else if name == "" || utf8.RuneCountInString(name) > maxMerchantRunes {
			errs = append(errs, entryErr(i, "merchant", "data.merchant_invalid", "the merchant must be 1 to 200 characters", "limit", fmt.Sprint(maxMerchantRunes)))
		} else {
			e.Merchant, rec.Merchant = name, name
		}
	default:
		errs = append(errs, entryErr(i, "merchant", "data.merchant_invalid", "the merchant must be 1 to 200 characters", "limit", fmt.Sprint(maxMerchantRunes)))
	}

	switch v := fields["total"].(type) {
	case nil:
		errs = append(errs, entryErr(i, "total", "data.field_missing", "total is required", "field", "total"))
	case json.Number:
		errs = append(errs, totalCents(i, v.String(), &e, &rec)...)
	case string:
		errs = append(errs, totalCents(i, strings.TrimSpace(v), &e, &rec)...)
	default:
		errs = append(errs, entryErr(i, "total", "data.total_invalid", "the total must be a decimal amount in EUR, e.g. 12.50"))
	}

	switch v := fields["extra"].(type) {
	case nil:
	case map[string]any:
		clean, extraErrs := cleanExtra(i, v)
		errs = append(errs, extraErrs...)
		if len(v) > 0 {
			rec.Extra = clean
		}
	default:
		errs = append(errs, entryErr(i, "extra", "data.extra_invalid", "extra must be an object"))
	}
	if len(errs) > 0 {
		return DataEntry{}, errs
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rec); err != nil {
		return DataEntry{}, []EntryError{entryErr(i, "extra", "data.extra_invalid", "extra could not be stored")}
	}
	e.Canonical = bytes.TrimRight(buf.Bytes(), "\n")
	if len(e.Canonical) > maxDataEntryBytes {
		return DataEntry{}, []EntryError{entryErr(i, "extra", "data.entry_too_large", "the entry is too large", "limitKb", fmt.Sprint(maxDataEntryBytes>>10))}
	}
	return e, nil
}

// totalCents converts a plain decimal EUR amount to cents without floats.
// Digits beyond the second decimal are rounded half up.
func totalCents(i int, s string, e *DataEntry, rec *canonicalRecord) []EntryError {
	bad := []EntryError{entryErr(i, "total", "data.total_invalid", "the total must be a decimal amount in EUR, e.g. 12.50")}
	whole, frac, _ := strings.Cut(s, ".")
	digits := func(s string) bool {
		for _, r := range s {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	if whole == "" || !digits(whole) || !digits(frac) || strings.Contains(s, ".") && frac == "" {
		return bad
	}
	whole = strings.TrimLeft(whole, "0")
	if len(whole) > 7 || len(frac) > 12 {
		return []EntryError{entryErr(i, "total", "data.total_range", "the total must be between 0.01 and 1000.00 EUR")}
	}
	frac += "000"
	var cents int64
	for _, r := range whole + frac[:2] {
		cents = cents*10 + int64(r-'0')
	}
	if frac[2] >= '5' {
		cents++
	}
	if cents < 1 || cents > maxReceiptCents {
		return []EntryError{entryErr(i, "total", "data.total_range", "the total must be between 0.01 and 1000.00 EUR")}
	}
	e.Total = money.Cents(cents)
	rec.Total = e.Total.String()
	return nil
}
