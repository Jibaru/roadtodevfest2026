package domain

// TitleHasKana reports whether a title contains hiragana or katakana —
// an unambiguous Japanese signal (kana exists only in Japanese), strong
// enough to override any other language guess.
func TitleHasKana(title string) bool {
	for _, c := range title {
		if (c >= 0x3040 && c <= 0x309f) || (c >= 0x30a0 && c <= 0x30ff) {
			return true
		}
	}
	return false
}

// DetectLangFromTitle scores the title's Unicode script. Deterministic
// fallback when no LLM is available; returns "" for Latin-only titles
// (where en/es can't be told apart by script, and romanization is a
// no-op anyway).
func DetectLangFromTitle(title string) string {
	ja, ko := 0, 0
	for _, c := range title {
		switch {
		case c >= 0x3040 && c <= 0x309f: // hiragana
			ja += 2
		case c >= 0x30a0 && c <= 0x30ff: // katakana
			ja += 2
		case c >= 0xac00 && c <= 0xd7a3: // hangul syllables
			ko += 2
		case c >= 0x1100 && c <= 0x11ff: // hangul jamo
			ko++
		case c >= 0x3130 && c <= 0x318f: // hangul compat jamo
			ko++
		case c >= 0x4e00 && c <= 0x9fff: // CJK ideographs: treat as ja in our domain
			ja++
		}
	}
	if ko >= 2 && ko >= ja {
		return "ko"
	}
	if ja >= 2 {
		return "ja"
	}
	return ""
}

// NeedsRomanization reports whether lyrics in this language need a
// Latin-script transliteration for the karaoke display.
func NeedsRomanization(language string) bool {
	switch language {
	case "ja", "japanese", "ko", "korean":
		return true
	}
	return false
}
