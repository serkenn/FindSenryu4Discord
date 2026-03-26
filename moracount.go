package main

import (
	"strings"
	"unicode"

	"github.com/ikawaha/kagome-dict/uni"
	"github.com/ikawaha/kagome/v2/tokenizer"

	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
)

// moraTokenizer is the kagome tokenizer used for fallback mora counting.
var moraTokenizer *tokenizer.Tokenizer

// initMoraTokenizer initializes the kagome tokenizer for mora counting.
func initMoraTokenizer() {
	t, err := tokenizer.New(uni.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		logger.Warn("Failed to initialize mora tokenizer for fallback detection", "error", err)
		return
	}
	moraTokenizer = t
}

// countMorae counts the number of morae (音) in a Japanese reading string.
// Rules:
//   - Each kana character = 1 mora
//   - ゃ・ゅ・ょ (拗音) do NOT count as separate morae (combine with previous)
//   - っ (促音) counts as 1 mora
//   - ー (長音) counts as 1 mora
//   - ん/ン counts as 1 mora
func countMorae(reading string) int {
	runes := []rune(reading)
	count := 0
	for i, r := range runes {
		_ = i
		// Skip small ya/yu/yo (拗音) — they combine with the previous character
		if isYouon(r) {
			continue
		}
		// Count katakana/hiragana characters
		if isKana(r) || r == 'ー' {
			count++
		}
	}
	return count
}

// isYouon returns true for small ya/yu/yo kana (拗音).
func isYouon(r rune) bool {
	switch r {
	case 'ゃ', 'ゅ', 'ょ', 'ャ', 'ュ', 'ョ':
		return true
	// Also handle small wa/wi/we/wo/ka/ke
	case 'ゎ', 'ヮ':
		return true
	}
	return false
}

// isKana returns true if the rune is hiragana or katakana.
func isKana(r rune) bool {
	return unicode.In(r, unicode.Hiragana, unicode.Katakana)
}

// getReading returns the katakana reading of a text using kagome.
func getReading(text string) string {
	if moraTokenizer == nil {
		return ""
	}
	tokens := moraTokenizer.Tokenize(text)
	var reading strings.Builder
	for _, token := range tokens {
		features := token.Features()
		if len(features) >= 8 && features[7] != "*" {
			// Use the reading (読み) field — index 7 in UniDic
			reading.WriteString(features[7])
		} else if len(features) >= 7 && features[6] != "*" {
			// Fallback to pronunciation field
			reading.WriteString(features[6])
		} else {
			// If no reading available, use surface form
			reading.WriteString(token.Surface)
		}
	}
	return reading.String()
}

// fallbackHaikuDetect tries to detect 5-7-5 or 5-7-5-7-7 patterns
// using kagome morphological analysis directly, as a fallback when go-haiku fails.
// Returns the split phrases and pattern type, or nil if no match.
func fallbackHaikuDetect(content string, rule []int) []string {
	if moraTokenizer == nil {
		return nil
	}

	tokens := moraTokenizer.Tokenize(content)

	var infos []tokenInfo
	for _, token := range tokens {
		surface := token.Surface
		features := token.Features()

		var reading string
		if len(features) >= 8 && features[7] != "*" {
			reading = features[7]
		} else if len(features) >= 7 && features[6] != "*" {
			reading = features[6]
		} else {
			// Convert surface to best-guess reading
			reading = surface
		}

		morae := countMorae(reading)
		if morae == 0 {
			// Skip punctuation, spaces, etc.
			// But if it's a non-kana character (like numbers), try to count it
			r := []rune(surface)
			if len(r) > 0 && !unicode.IsPunct(r[0]) && !unicode.IsSpace(r[0]) {
				morae = len(r) // fallback: 1 mora per character
			}
		}
		if morae > 0 {
			infos = append(infos, tokenInfo{surface: surface, reading: reading, morae: morae})
		}
	}

	if len(infos) == 0 {
		return nil
	}

	// Try to split tokens into the given mora pattern
	return trySplitByMorae(infos, rule)
}

// tokenInfo holds token analysis results.
type tokenInfo struct {
	surface string
	reading string
	morae   int
}

// trySplitByMorae attempts to split tokenized text into phrase groups matching the mora pattern.
func trySplitByMorae(infos []tokenInfo, rule []int) []string {
	phrases := make([]string, 0, len(rule))
	tokenIdx := 0
	for _, target := range rule {
		moraeSum := 0
		var phraseParts []string
		for tokenIdx < len(infos) && moraeSum < target {
			info := infos[tokenIdx]
			remaining := target - moraeSum
			if info.morae <= remaining {
				phraseParts = append(phraseParts, info.surface)
				moraeSum += info.morae
				tokenIdx++
			} else {
				// Token has more morae than needed — can't split cleanly
				return nil
			}
		}
		if moraeSum != target {
			return nil
		}
		phrases = append(phrases, strings.Join(phraseParts, ""))
	}

	// All tokens must be consumed
	if tokenIdx != len(infos) {
		return nil
	}

	return phrases
}
