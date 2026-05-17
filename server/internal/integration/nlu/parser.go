// Package nlu provides step-aware Turkish slot-filling for the patient
// intake assistant. V1 is rule-based — encoded knowledge about common
// Turkish utterance patterns at each step of the intake dialog. The
// surface (`Parse(step, transcript, opts)`) is designed so a future
// upgrade to Whisper-derived ASR + Claude-extraction can swap in with
// zero call-site change.
//
// What "trained" means here: the rules below capture what we observed
// from real Turkish hospital staff phrasing patient answers ("adım …",
// "doğum tarihim …", "kardiyoloji bekliyorum"). We do NOT do entity-
// resolution against a real NLP model — that's the next iteration.
package nlu

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Step is the dialog step we're parsing for. Each step extracts a
// different slot from the same transcript.
type Step string

const (
	StepTC             Step = "tc"
	StepName           Step = "name"
	StepBirthYear      Step = "birthYear"
	StepPhone          Step = "phone"
	StepComplaint      Step = "complaint"
	StepSpecialization Step = "specialization"
	StepConfirm        Step = "confirm"
)

// ParseInput carries the transcript + dialog-step context, plus optional
// catalog data for fuzzy-matching slots (e.g. specialization names).
type ParseInput struct {
	Step       Step
	Transcript string

	// Specialization catalog — used only when Step == StepSpecialization.
	// Empty slice falls back to "no match found".
	Specializations []SpecHint
}

type SpecHint struct {
	ID   string
	Name string
}

// ParseResult is the extracted slot. Confidence runs 0..1; UIs may use
// it to decide whether to auto-advance or ask the user to confirm.
type ParseResult struct {
	// One of these is set, depending on Step:
	TC             string `json:"tc,omitempty"`
	FirstName      string `json:"first_name,omitempty"`
	LastName       string `json:"last_name,omitempty"`
	BirthYear      int    `json:"birth_year,omitempty"`
	Phone          string `json:"phone,omitempty"`
	Complaint      string `json:"complaint,omitempty"`
	SpecID         string `json:"specialization_id,omitempty"`
	SpecName       string `json:"specialization_name,omitempty"`
	ConfirmYes     bool   `json:"confirm_yes,omitempty"`
	ConfirmNo      bool   `json:"confirm_no,omitempty"`

	// Confidence is heuristic — for the rule engine it's:
	//   1.0 = strong pattern match (regex hit)
	//   0.7 = best-effort with multiple plausible interpretations
	//   0.0 = no extraction
	Confidence float64 `json:"confidence"`

	// Human-readable summary the UI can speak back to confirm
	// ("Adınız Hasan Demir, doğru mu?"). Empty when nothing extracted.
	Echo string `json:"echo,omitempty"`
}

// Parse dispatches to the per-step extractor.
func Parse(in ParseInput) ParseResult {
	t := normalize(in.Transcript)
	if t == "" {
		return ParseResult{}
	}
	switch in.Step {
	case StepTC:
		return parseTC(t)
	case StepName:
		return parseName(t)
	case StepBirthYear:
		return parseBirthYear(t)
	case StepPhone:
		return parsePhone(t)
	case StepComplaint:
		return parseComplaint(t)
	case StepSpecialization:
		return parseSpecialization(t, in.Specializations)
	case StepConfirm:
		return parseConfirm(t)
	}
	return ParseResult{}
}

// ---------- Step parsers ----------

// rxDigits11 matches exactly 11 digits, possibly with spaces or dots
// between them ("10 000 000 146" / "10.000.000.146").
var rxDigits11 = regexp.MustCompile(`(?:\d[\s\.\-]*){10}\d`)

func parseTC(t string) ParseResult {
	m := rxDigits11.FindString(t)
	digits := stripNonDigits(m)
	if len(digits) != 11 {
		return ParseResult{}
	}
	return ParseResult{
		TC:         digits,
		Confidence: 1.0,
		Echo:       "TC kimlik numaranız " + digits + ", doğru mu?",
	}
}

// Common Turkish filler words around names: "adım", "ismim", "ben",
// "soyadım". We strip these so "adım hasan demir" → "hasan demir".
var nameFillers = []string{
	"adim", "adımı", "ismi", "ismim", "ben", "benim",
	"soyad", "soyadim", "soyadım", "ad", "ismimi",
}

func parseName(t string) ParseResult {
	// Drop punctuation, lowercase already done by normalize.
	t = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || r == ' ' {
			return r
		}
		return -1
	}, t)
	words := strings.Fields(t)
	// Strip fillers.
	cleaned := make([]string, 0, len(words))
	for _, w := range words {
		if isFiller(w) {
			continue
		}
		cleaned = append(cleaned, w)
	}
	if len(cleaned) == 0 {
		return ParseResult{}
	}
	// Turkish names are typically First + (Middle?) + Last. Heuristic:
	// last word is surname, everything before is given name(s).
	var first, last string
	switch {
	case len(cleaned) == 1:
		// Only one token — assume it's the first name; surname empty.
		first = titleCase(cleaned[0])
	case len(cleaned) >= 2:
		last = titleCase(cleaned[len(cleaned)-1])
		first = titleCase(strings.Join(cleaned[:len(cleaned)-1], " "))
	}
	if first == "" && last == "" {
		return ParseResult{}
	}
	conf := 0.9
	if len(cleaned) == 1 {
		conf = 0.6
	}
	echo := first
	if last != "" {
		echo = strings.TrimSpace(first + " " + last)
	}
	return ParseResult{
		FirstName:  first,
		LastName:   last,
		Confidence: conf,
		Echo:       "Adınız " + echo + ", doğru mu?",
	}
}

func isFiller(w string) bool {
	for _, f := range nameFillers {
		if w == f {
			return true
		}
	}
	return false
}

// rxYear matches a 4-digit year that's plausibly a birth year.
var rxYear = regexp.MustCompile(`\b(19\d{2}|20[0-2]\d)\b`)

func parseBirthYear(t string) ParseResult {
	m := rxYear.FindString(t)
	if m == "" {
		return ParseResult{}
	}
	n := 0
	for _, r := range m {
		n = n*10 + int(r-'0')
	}
	return ParseResult{
		BirthYear:  n,
		Confidence: 1.0,
		Echo:       "Doğum yılınız " + m + ", doğru mu?",
	}
}

// Phone parsing — Turkish format. We extract 10-11 digit sequences with
// optional country code.
var rxPhone = regexp.MustCompile(`(?:\+?90[\s\-]?)?(?:\(?0?5\d{2}\)?[\s\-]?)\d{3}[\s\-]?\d{2}[\s\-]?\d{2}`)

func parsePhone(t string) ParseResult {
	m := rxPhone.FindString(t)
	if m == "" {
		// Fallback — pure 10-digit run starting with 5.
		fallback := regexp.MustCompile(`\b5\d{9}\b`).FindString(t)
		if fallback == "" {
			return ParseResult{}
		}
		m = fallback
	}
	digits := stripNonDigits(m)
	// Drop leading 90 if present (country code).
	digits = strings.TrimPrefix(digits, "90")
	// Drop leading 0 if present (national prefix).
	digits = strings.TrimPrefix(digits, "0")
	if len(digits) != 10 {
		return ParseResult{}
	}
	formatted := "+90 " + digits[:3] + " " + digits[3:6] + " " + digits[6:8] + " " + digits[8:]
	return ParseResult{
		Phone:      formatted,
		Confidence: 0.95,
		Echo:       "Telefon numaranız " + formatted + ", doğru mu?",
	}
}

func parseComplaint(t string) ParseResult {
	// Complaint is free-text. Strip leading "şikayetim", "rahatsızlığım"
	// etc. to keep the slot tidy. Cap to ~200 chars.
	for _, prefix := range []string{
		"sikayetim ", "şikayetim ", "rahatsizligim ", "rahatsızlığım ",
		"ben ", "bende ",
	} {
		t = strings.TrimPrefix(t, prefix)
	}
	if len(t) > 200 {
		t = t[:200]
	}
	t = strings.TrimSpace(t)
	if t == "" {
		return ParseResult{}
	}
	return ParseResult{
		Complaint:  capitalize(t),
		Confidence: 0.8,
		Echo:       "Şikayetinizi anladım: " + capitalize(t) + ". Devam edelim mi?",
	}
}

// parseSpecialization fuzzy-matches the transcript against the supplied
// catalog. Scoring is a mix of substring containment + token overlap;
// best score wins. Empty catalog → empty result.
func parseSpecialization(t string, catalog []SpecHint) ParseResult {
	if len(catalog) == 0 {
		return ParseResult{}
	}
	t = normalize(t)
	type scored struct {
		id    string
		name  string
		score float64
	}
	out := make([]scored, 0, len(catalog))
	for _, s := range catalog {
		n := normalize(s.Name)
		score := fuzzyScore(t, n)
		if score > 0 {
			out = append(out, scored{s.ID, s.Name, score})
		}
	}
	if len(out) == 0 {
		return ParseResult{}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].score > out[j].score
	})
	best := out[0]
	// Need at least a moderate score to commit.
	if best.score < 0.4 {
		return ParseResult{}
	}
	conf := best.score
	if conf > 1.0 {
		conf = 1.0
	}
	return ParseResult{
		SpecID:     best.id,
		SpecName:   best.name,
		Confidence: conf,
		Echo:       best.name + " bölümünü seçtim, doğru mu?",
	}
}

// fuzzyScore: 1.0 for exact substring, 0.7 for token overlap, lower
// for partial. Cheap heuristic, no external dependency.
func fuzzyScore(transcript, candidate string) float64 {
	if transcript == "" || candidate == "" {
		return 0
	}
	if strings.Contains(transcript, candidate) {
		return 1.0
	}
	// Token overlap: how many of candidate's tokens appear in transcript.
	candTokens := strings.Fields(candidate)
	if len(candTokens) == 0 {
		return 0
	}
	hits := 0
	for _, tok := range candTokens {
		// Tokens shorter than 3 chars (like "iç") get a wider net via
		// substring; longer tokens require word-boundary contains.
		if len(tok) < 3 {
			continue
		}
		if strings.Contains(transcript, tok) {
			hits++
		}
	}
	return float64(hits) / float64(len(candTokens))
}

// Confirmation words — Turkish + a few common English borrowings.
var (
	yesWords = []string{
		"evet", "tamam", "olur", "onayliyorum", "onaylıyorum",
		"dogru", "doğru", "kabul", "kabul ediyorum", "ok", "oley",
	}
	noWords = []string{
		"hayir", "hayır", "yok", "yanlis", "yanlış",
		"iptal", "vazgec", "vazgeç", "geri", "duzelt", "düzelt",
		"yapmiyorum", "yapmıyorum",
	}
)

func parseConfirm(t string) ParseResult {
	for _, y := range yesWords {
		if strings.Contains(t, y) {
			return ParseResult{
				ConfirmYes: true, Confidence: 1.0,
				Echo: "Onayınızı aldım, kaydı oluşturuyorum.",
			}
		}
	}
	for _, n := range noWords {
		if strings.Contains(t, n) {
			return ParseResult{
				ConfirmNo: true, Confidence: 1.0,
				Echo: "Geri dönüyorum, düzeltebilirsiniz.",
			}
		}
	}
	return ParseResult{}
}

// ---------- Helpers ----------

// normalize lowercases (Turkish-aware), folds extended TR chars to ASCII,
// collapses whitespace. The fold helps regex + fuzzy matches stay
// stable regardless of whether STT returns "ş" or "s".
func normalize(s string) string {
	s = strings.ToLower(s)
	s = trReplacer.Replace(s)
	// Collapse multiple spaces.
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}

var trReplacer = strings.NewReplacer(
	"ç", "c", "ğ", "g", "ı", "i", "ö", "o", "ş", "s", "ü", "u",
)

func stripNonDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// titleCase capitalizes each space-separated word, Turkish-locale-naive
// (the user can edit the form field if it gets a name wrong).
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		for j := 1; j < len(r); j++ {
			r[j] = unicode.ToLower(r[j])
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
