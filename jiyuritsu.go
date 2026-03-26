package main

import (
	"strings"
)

// jiyuritsuEntry represents a famous free-form haiku for whitelist matching.
type jiyuritsuEntry struct {
	Text   string // the poem text
	Author string // the poet
}

// jiyuritsuWhitelist contains famous free-form haiku (自由律俳句) and other
// well-known poems that don't follow the standard 5-7-5 pattern.
// When a user's message contains one of these, it is detected and attributed.
var jiyuritsuWhitelist = []jiyuritsuEntry{
	// ===== 尾崎放哉 (Ozaki Hosai) =====
	{"咳をしても一人", "尾崎放哉"},
	{"入れものが無い両手で受ける", "尾崎放哉"},
	{"墓のうらに廻る", "尾崎放哉"},
	{"足のうら洗へば白くなる", "尾崎放哉"},
	{"こんなよい月を一人で見て寝る", "尾崎放哉"},
	{"濡れて来て猫が横切る", "尾崎放哉"},
	{"何も持たない手を振って来る", "尾崎放哉"},
	{"鳥が啼く方の山から暮れる", "尾崎放哉"},
	{"肉がやせて来る太い骨である", "尾崎放哉"},
	{"春の山のうしろから烟が出だした", "尾崎放哉"},
	{"一つ蛍にされてゐる闇", "尾崎放哉"},
	{"障子しめきって淋しさをみたす", "尾崎放哉"},
	{"月が昇って何を待つでもなく", "尾崎放哉"},
	{"花いちもんめの声が夕焼ける", "尾崎放哉"},
	{"渚白い足出す", "尾崎放哉"},

	// ===== 種田山頭火 (Taneda Santoka) =====
	{"分け入っても分け入っても青い山", "種田山頭火"},
	{"まっすぐな道でさみしい", "種田山頭火"},
	{"鉄鉢の中へも霰", "種田山頭火"},
	{"うしろすがたのしぐれてゆくか", "種田山頭火"},
	{"どうしようもないわたしが歩いてゐる", "種田山頭火"},
	{"どうしようもないわたしが歩いている", "種田山頭火"},
	{"一人の道が暮れて来た", "種田山頭火"},
	{"雨だれの音も年とった", "種田山頭火"},
	{"また見ることもない山が遠ざかる", "種田山頭火"},
	{"炎天をいただいて乞ひ歩く", "種田山頭火"},
	{"笠にとんぼをとまらせてあるく", "種田山頭火"},
	{"しぐるるや死なないでゐる", "種田山頭火"},
	{"濁れる水のながれつつ澄む", "種田山頭火"},
	{"生死の中の雪ふりしきる", "種田山頭火"},
	{"水のきれいな町に来た", "種田山頭火"},
	{"もりもり盛り上がる雲へ歩む", "種田山頭火"},
	{"あるけばかつこういそげばかつこう", "種田山頭火"},
	{"ふくろうはふくろうでわたしはわたしでねむれない", "種田山頭火"},
	{"この旅果てもない旅のつくつくぼうし", "種田山頭火"},
	{"酔うてこほろぎと寝ていたよ", "種田山頭火"},
	{"捨てきれない荷物のおもさまへうしろ", "種田山頭火"},
	{"鴉啼いてわたしも一人", "種田山頭火"},

	// ===== 松尾芭蕉 (Matsuo Basho) =====
	{"閑さや岩にしみ入る蝉の声", "松尾芭蕉"},
	{"しずかさや岩にしみ入る蝉の声", "松尾芭蕉"},

	// ===== 荻原井泉水 (Ogiwara Seisensui) =====
	{"陽へ病む", "荻原井泉水"},
	{"よい湯からよい月が出た", "荻原井泉水"},
	{"月光の下に蹲る", "荻原井泉水"},

	// ===== 河東碧梧桐 (Kawahigashi Hekigoto) =====
	{"赤い椿白い椿と落ちにけり", "河東碧梧桐"},

	// ===== 住宅顕信 (Jutaku Kenshin) =====
	{"ずぶぬれて犬ころ", "住宅顕信"},
	{"若さとはこんな淋しい春なのか", "住宅顕信"},

	// ===== 栗林一石路 (Kuribayashi Issekiro) =====
	{"タバコの火が暗い方へ歩いてゐる", "栗林一石路"},

	// ===== 橋本夢道 (Hashimoto Mudo) =====
	{"無礼なるバットを持って立ちにけり", "橋本夢道"},

	// ===== 中塚一碧楼 (Nakatsuka Ippekiro) =====
	{"昼の虫よけて歩く僧正", "中塚一碧楼"},

}

// normalizeForMatch normalizes text for fuzzy matching:
// removes spaces, converts to lowercase, normalizes numbers and kana variations.
func normalizeForMatch(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "　", "")
	s = strings.ReplaceAll(s, "\t", "")
	// Normalize common character variations
	s = strings.ReplaceAll(s, "ゐ", "い")
	s = strings.ReplaceAll(s, "ゑ", "え")
	s = strings.ReplaceAll(s, "ヰ", "イ")
	s = strings.ReplaceAll(s, "ヱ", "エ")
	s = strings.ReplaceAll(s, "づ", "ず")
	s = strings.ReplaceAll(s, "ぢ", "じ")
	// Normalize all number forms (kanji, full-width, half-width) to "0"
	s = normalizeNumbers(s)
	return s
}

// normalizeNumbers replaces all numeric representations with "0" for fuzzy matching.
func normalizeNumbers(s string) string {
	var result []rune
	for _, r := range s {
		if isNumericRune(r) {
			result = append(result, '0')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// isNumericRune returns true for any numeric character:
// half-width digits, full-width digits, and kanji numerals.
func isNumericRune(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r >= '０' && r <= '９':
		return true
	}
	switch r {
	case '一', '二', '三', '四', '五', '六', '七', '八', '九', '十',
		'百', '千', '万', '億', '兆', '壱', '弐', '参':
		return true
	}
	return false
}

// matchJiyuritsu checks if the given message contains a known free-form haiku.
// Returns the matched entry or nil.
func matchJiyuritsu(content string) *jiyuritsuEntry {
	normalized := normalizeForMatch(content)

	for i := range jiyuritsuWhitelist {
		entry := &jiyuritsuWhitelist[i]
		target := normalizeForMatch(entry.Text)
		if strings.Contains(normalized, target) {
			return entry
		}
	}
	return nil
}
