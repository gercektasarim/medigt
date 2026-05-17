// Package hl7 parses HL7 v2.x ORU^R01 (Observation Result) messages —
// the standard envelope lab autoanalizörs use to push results into HBYS.
//
// We parse a minimal subset:
//
//	MSH  — message header (carries delimiters)
//	PID  — patient (MRN read from field 3)
//	OBR  — observation request (filler/placer order ids in fields 2-3)
//	OBX  — observation result (one per test result line)
//
// Other segments (NTE, NK1, PV1, ZDS …) are tolerated but ignored. Real
// vendor messages routinely include them.
//
// Delimiters per HL7 spec:
//   field    = '|'  segment field separator
//   comp     = '^'  component separator within a field
//   subcomp  = '&'  sub-component separator
//   rep      = '~'  field repetition separator
//   escape   = '\'  escape character
//
// MSH-1 holds the field separator literally; MSH-2 holds the rest in the
// order comp/rep/escape/subcomp. We honour the message's declared
// delimiters even though most senders default to "|^~\&".
package hl7

import (
	"errors"
	"strings"
	"time"
)

// Message is the parsed ORU^R01 in HBYS-friendly shape. Only the bits we
// act on (patient MRN, order ids, observations) are populated; the raw
// payload is kept for audit.
type Message struct {
	MessageControlID string
	SentAt           time.Time

	PatientMRN string
	PatientLastName string
	PatientFirstName string

	PlacerOrderNo string // OBR-2 — our system's order_no
	FillerOrderNo string // OBR-3 — the analyzer's accession

	Observations []Observation

	Raw string
}

// Observation is one OBX line — typically one analyte (HGB, GLU, …).
type Observation struct {
	ValueType   string // NM (numeric), ST (string), CE (coded), TX (text)
	TestCode    string // "HGB" / "GLU" — our lab_test_catalog.code
	TestName    string // free-text component-2 of OBX-3
	Value       string // raw value as text
	NumericValue *float64
	Unit        string
	ReferenceRange string
	AbnormalFlag string // L (low), H (high), HH (critical high), LL, N, A
	Status      string // F (final), P (preliminary), C (corrected)
}

var (
	ErrEmpty           = errors.New("HL7 mesajı boş")
	ErrNotORU          = errors.New("yalnızca ORU^R01 desteklenir")
	ErrMissingMSH      = errors.New("MSH segmenti eksik")
	ErrInvalidEncoding = errors.New("MSH-2 ayraçları okunamadı")
)

// ParseORU parses a single HL7 ORU^R01 message into Message.
// Accepts CR / LF / CRLF segment terminators.
func ParseORU(raw string) (*Message, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, ErrEmpty
	}
	// Normalise segment terminator: HL7 wire format uses \r; allow \n too.
	raw = strings.ReplaceAll(raw, "\r\n", "\r")
	raw = strings.ReplaceAll(raw, "\n", "\r")
	segments := strings.Split(raw, "\r")

	// MSH parsing — declares delimiters.
	var msh string
	for _, s := range segments {
		if strings.HasPrefix(s, "MSH") {
			msh = s
			break
		}
	}
	if msh == "" {
		return nil, ErrMissingMSH
	}
	if len(msh) < 8 {
		return nil, ErrInvalidEncoding
	}
	fieldSep := string(msh[3])
	// Encoding chars at MSH-2 — typically "^~\&".
	encoding := msh[4:]
	endIdx := strings.IndexByte(encoding, msh[3])
	if endIdx < 0 || endIdx < 4 {
		return nil, ErrInvalidEncoding
	}
	enc := encoding[:endIdx]
	compSep := string(enc[0])
	// rep := string(enc[1])
	// escape := string(enc[2])
	// subSep := string(enc[3])

	msg := &Message{Raw: raw}

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		fields := strings.Split(seg, fieldSep)
		segType := fields[0]
		switch segType {
		case "MSH":
			// MSH fields: MSH-9 (index 8 after split where MSH itself sits
			// at 0 because field separator is the 1st char). Note: in MSH
			// only, the first field separator IS field 1 — so
			// MSH-9 = fields[9] when we count from "MSH" + field-sep.
			if len(fields) > 9 {
				msgType := fields[8]
				if !strings.HasPrefix(msgType, "ORU"+compSep+"R01") &&
					!strings.HasPrefix(msgType, "ORU") {
					return nil, ErrNotORU
				}
			}
			if len(fields) > 9 {
				msg.MessageControlID = fields[9]
			}
			if len(fields) > 6 {
				if t, err := parseHL7Time(fields[6]); err == nil {
					msg.SentAt = t
				}
			}
		case "PID":
			// PID-3: MRN (may be repeated: split by '~'); take first.
			if len(fields) > 3 {
				mrnField := fields[3]
				if i := strings.IndexByte(mrnField, '~'); i >= 0 {
					mrnField = mrnField[:i]
				}
				// First component is the ID.
				if i := strings.IndexByte(mrnField, compSep[0]); i >= 0 {
					mrnField = mrnField[:i]
				}
				msg.PatientMRN = strings.TrimSpace(mrnField)
			}
			// PID-5: family^given^middle …
			if len(fields) > 5 {
				comps := strings.Split(fields[5], compSep)
				if len(comps) > 0 {
					msg.PatientLastName = strings.TrimSpace(comps[0])
				}
				if len(comps) > 1 {
					msg.PatientFirstName = strings.TrimSpace(comps[1])
				}
			}
		case "OBR":
			if len(fields) > 2 {
				msg.PlacerOrderNo = strings.TrimSpace(stripCompSuffix(fields[2], compSep))
			}
			if len(fields) > 3 {
				msg.FillerOrderNo = strings.TrimSpace(stripCompSuffix(fields[3], compSep))
			}
		case "OBX":
			obs := parseOBX(fields, compSep)
			msg.Observations = append(msg.Observations, obs)
		}
	}

	return msg, nil
}

func parseOBX(fields []string, compSep string) Observation {
	obs := Observation{}
	get := func(i int) string {
		if i < len(fields) {
			return strings.TrimSpace(fields[i])
		}
		return ""
	}
	obs.ValueType = get(2)
	// OBX-3: code^name^coding-system
	id := get(3)
	if id != "" {
		parts := strings.Split(id, compSep)
		obs.TestCode = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			obs.TestName = strings.TrimSpace(parts[1])
		}
	}
	obs.Value = get(5)
	obs.Unit = get(6)
	obs.ReferenceRange = get(7)
	obs.AbnormalFlag = get(8)
	obs.Status = get(11)

	// For numeric values, parse to a float for downstream comparisons.
	if obs.ValueType == "NM" || obs.ValueType == "" {
		if f, ok := parseFloat(obs.Value); ok {
			obs.NumericValue = &f
		}
	}
	return obs
}

func stripCompSuffix(s, compSep string) string {
	if i := strings.Index(s, compSep); i >= 0 {
		return s[:i]
	}
	return s
}

// parseHL7Time accepts YYYYMMDD, YYYYMMDDHHMM, YYYYMMDDHHMMSS forms.
func parseHL7Time(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty")
	}
	layouts := []string{"20060102", "200601021504", "20060102150405"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("HL7 zaman damgası anlaşılamadı")
}

func parseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// Accept comma as decimal separator (TR locale)
	s = strings.ReplaceAll(s, ",", ".")
	// Tolerant scan
	var sign float64 = 1
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		if s[i] == '-' {
			sign = -1
		}
		i++
	}
	if i >= len(s) {
		return 0, false
	}
	var intPart, fracPart int64
	var fracDigits int64 = 1
	state := 0 // 0=int, 1=frac
	for ; i < len(s); i++ {
		c := s[i]
		if c == '.' && state == 0 {
			state = 1
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		if state == 0 {
			intPart = intPart*10 + int64(c-'0')
		} else {
			fracPart = fracPart*10 + int64(c-'0')
			fracDigits *= 10
		}
	}
	v := float64(intPart) + float64(fracPart)/float64(fracDigits)
	return sign * v, true
}
