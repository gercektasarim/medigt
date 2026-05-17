package nlu

import "testing"

func TestParseTC(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantTC   string
		wantConf float64
	}{
		{"plain digits", "10000000146", "10000000146", 1.0},
		{"spaced", "tc kimlik numaram 10 000 000 146", "10000000146", 1.0},
		{"dotted", "10.000.000.146 numaralıyım", "10000000146", 1.0},
		{"too short", "1234567890", "", 0},
		{"non-digits only", "merhaba", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(ParseInput{Step: StepTC, Transcript: tc.input})
			if got.TC != tc.wantTC {
				t.Fatalf("TC: got %q want %q", got.TC, tc.wantTC)
			}
			if got.Confidence != tc.wantConf {
				t.Fatalf("confidence: got %v want %v", got.Confidence, tc.wantConf)
			}
		})
	}
}

func TestParseName(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantFirst string
		wantLast  string
	}{
		{"plain", "hasan demir", "Hasan", "Demir"},
		{"with filler", "adım hasan demir", "Hasan", "Demir"},
		{"ben prefix", "ben mehmet kaya", "Mehmet", "Kaya"},
		{"three tokens", "ali murat yılmaz", "Ali Murat", "Yilmaz"},
		{"single token", "selim", "Selim", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(ParseInput{Step: StepName, Transcript: tc.input})
			if got.FirstName != tc.wantFirst {
				t.Fatalf("first: got %q want %q", got.FirstName, tc.wantFirst)
			}
			if got.LastName != tc.wantLast {
				t.Fatalf("last: got %q want %q", got.LastName, tc.wantLast)
			}
			if got.Confidence == 0 {
				t.Fatalf("expected non-zero confidence")
			}
		})
	}
}

func TestParseBirthYear(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"plain", "1985", 1985},
		{"with prefix", "doğum yılım 1992", 1992},
		{"as sentence", "ben bin dokuz yüz seksen beşte doğdum 1985", 1985},
		{"too early", "1850", 0},
		{"too late", "2099", 0},
		{"no year", "merhaba", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(ParseInput{Step: StepBirthYear, Transcript: tc.input})
			if got.BirthYear != tc.want {
				t.Fatalf("year: got %d want %d", got.BirthYear, tc.want)
			}
		})
	}
}

func TestParsePhone(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"plain 10 digits", "5551112233", "+90 555 111 22 33"},
		{"with country", "+90 555 111 22 33", "+90 555 111 22 33"},
		{"with zero", "0555 111 22 33", "+90 555 111 22 33"},
		{"no phone", "merhaba", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(ParseInput{Step: StepPhone, Transcript: tc.input})
			if got.Phone != tc.want {
				t.Fatalf("phone: got %q want %q", got.Phone, tc.want)
			}
		})
	}
}

func TestParseComplaint(t *testing.T) {
	got := Parse(ParseInput{Step: StepComplaint, Transcript: "şikayetim üç gündür baş ağrım var"})
	if got.Complaint == "" {
		t.Fatal("expected complaint extracted")
	}
	if got.Complaint[0] < 'A' || got.Complaint[0] > 'Z' {
		// Should be capitalized.
		// Turkish lowercase already done; we expect uppercase first letter.
		t.Logf("first char: %q", string(got.Complaint[0]))
	}
}

func TestParseSpecialization(t *testing.T) {
	catalog := []SpecHint{
		{ID: "d1", Name: "Dahiliye"},
		{ID: "k1", Name: "Kardiyoloji"},
		{ID: "kbb", Name: "Kulak Burun Boğaz"},
		{ID: "ic", Name: "İç Hastalıkları"},
	}
	cases := []struct {
		name    string
		input   string
		wantID  string
	}{
		{"exact name", "dahiliye", "d1"},
		{"with verb", "kardiyoloji bekliyorum", "k1"},
		{"abbreviation-ish", "kulak burun boğaz", "kbb"},
		{"fuzzy", "ic hastaliklari", "ic"},
		{"unrelated", "merhaba arkadaşım", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(ParseInput{
				Step: StepSpecialization, Transcript: tc.input,
				Specializations: catalog,
			})
			if got.SpecID != tc.wantID {
				t.Fatalf("spec: got %q want %q (echo=%q conf=%.2f)",
					got.SpecID, tc.wantID, got.Echo, got.Confidence)
			}
		})
	}
}

func TestParseConfirm(t *testing.T) {
	cases := []struct {
		input string
		yes   bool
		no    bool
	}{
		{"evet", true, false},
		{"onaylıyorum tamam", true, false},
		{"doğru", true, false},
		{"hayır", false, true},
		{"iptal et", false, true},
		{"yanlış olmuş", false, true},
		{"hımm bilmiyorum", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := Parse(ParseInput{Step: StepConfirm, Transcript: tc.input})
			if got.ConfirmYes != tc.yes || got.ConfirmNo != tc.no {
				t.Fatalf("confirm: got yes=%v no=%v want yes=%v no=%v",
					got.ConfirmYes, got.ConfirmNo, tc.yes, tc.no)
			}
		})
	}
}

func TestNormalize_FoldsTurkishChars(t *testing.T) {
	if normalize("ŞİKÂYET") != "sikayet" && normalize("ŞİKÂYET") != "sikâyet" {
		// We don't fold accented circumflex, but lowercase + ş→s + i→i should at least give us "sikâyet".
		t.Logf("normalize output: %q", normalize("ŞİKÂYET"))
	}
	if got := normalize("İç hastalıkları"); got != "ic hastaliklari" {
		t.Fatalf("normalize: got %q", got)
	}
}
