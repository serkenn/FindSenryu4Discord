package main

import "strings"

// Season represents the season associated with a kigo.
type Season int

const (
	SeasonSpring Season = iota // 春
	SeasonSummer               // 夏
	SeasonAutumn               // 秋
	SeasonWinter               // 冬
	SeasonNewYear              // 新年
)

// SeasonName returns the Japanese name for the season.
func (s Season) SeasonName() string {
	switch s {
	case SeasonSpring:
		return "春"
	case SeasonSummer:
		return "夏"
	case SeasonAutumn:
		return "秋"
	case SeasonWinter:
		return "冬"
	case SeasonNewYear:
		return "新年"
	}
	return ""
}

// SeasonEmoji returns a representative emoji for the season.
func (s Season) SeasonEmoji() string {
	switch s {
	case SeasonSpring:
		return "🌸"
	case SeasonSummer:
		return "🌻"
	case SeasonAutumn:
		return "🍁"
	case SeasonWinter:
		return "❄️"
	case SeasonNewYear:
		return "🎍"
	}
	return ""
}

// KigoResult holds the result of kigo detection.
type KigoResult struct {
	Word   string // the matched kigo
	Season Season // the season
}

// kigoMap maps kigo words to their season.
// Organized by season for maintainability.
var kigoMap map[string]Season

func init() {
	kigoMap = make(map[string]Season)

	// ===== 春 (Spring) =====
	springKigo := []string{
		// 時候
		"春", "立春", "早春", "浅春", "春浅し", "春めく", "仲春", "春分", "晩春",
		"暖か", "うららか", "麗か", "のどか", "花曇", "春暁", "春の朝", "春の昼",
		"春の夜", "春の宵", "朧月", "朧", "霞", "陽炎", "逃水",
		// 天文
		"春風", "東風", "春一番", "春雨", "春の雪", "春雷", "春の月", "朧月夜",
		"花冷え", "花曇り",
		// 地理
		"雪解", "雪崩", "春の水", "春の川", "春の海", "春泥", "春の山",
		// 生活
		"卒業", "入学", "花見", "雛祭", "ひな祭り", "雛", "潮干狩",
		"田植", "種蒔", "春耕",
		// 動物
		"鶯", "うぐいす", "雲雀", "ひばり", "燕", "つばめ", "蝶", "蝶々",
		"蛙", "蛙", "おたまじゃくし", "蜂",
		// 植物
		"桜", "さくら", "花", "梅", "椿", "菜の花", "菜花", "桃", "桃の花",
		"木蓮", "沈丁花", "蒲公英", "たんぽぽ", "土筆", "つくし", "蕨", "わらび",
		"蕗の薹", "ふきのとう", "若草", "若菜", "芽吹", "木の芽", "花吹雪",
		"花筏", "散る花", "葉桜", "藤", "牡丹", "躑躅", "つつじ", "芍薬",
		"菫", "すみれ", "チューリップ", "薔薇", "スミレ",
	}

	// ===== 夏 (Summer) =====
	summerKigo := []string{
		// 時候
		"夏", "立夏", "初夏", "梅雨", "梅雨入", "梅雨明", "盛夏", "炎天",
		"大暑", "暑し", "暑さ", "涼し", "涼", "夏の朝", "夏の夜", "短夜", "明易",
		"熱帯夜", "夏至", "土用", "晩夏",
		// 天文
		"夕立", "雷", "入道雲", "積乱雲", "虹", "夏の月", "夏の星", "天の川",
		"五月雨", "さみだれ",
		// 地理
		"滝", "清水", "泉", "夏の海", "夏の川", "夏野", "青田",
		// 生活
		"花火", "祭", "盆", "盂蘭盆", "七夕", "海水浴", "プール", "日焼",
		"扇風機", "冷房", "団扇", "うちわ", "風鈴", "蚊帳", "蚊取線香",
		"氷", "かき氷", "冷奴", "素麺", "そうめん", "西瓜", "すいか", "ビール",
		"夏休み", "林間学校", "甲子園",
		// 動物
		"蝉", "せみ", "蛍", "ほたる", "金魚", "蚊", "蝿", "蟻",
		"蜻蛉", "とんぼ", "カブトムシ", "クワガタ", "蛞蝓", "なめくじ",
		"郭公", "かっこう", "カッコウ", "時鳥", "ほととぎす",
		// 植物
		"向日葵", "ひまわり", "朝顔", "百合", "紫陽花", "あじさい",
		"蓮", "睡蓮", "茄子", "トマト", "胡瓜", "きゅうり", "枝豆",
		"青葉", "若葉", "新緑", "万緑", "夏草", "夏木立",
	}

	// ===== 秋 (Autumn) =====
	autumnKigo := []string{
		// 時候
		"秋", "立秋", "初秋", "仲秋", "晩秋", "秋分", "秋めく", "秋深し",
		"秋の朝", "秋の夜", "夜長", "秋の暮", "秋の宵", "釣瓶落し",
		"爽やか", "身に沁む", "肌寒",
		// 天文
		"秋風", "野分", "台風", "秋雨", "霧", "露", "秋の月", "名月",
		"月見", "十五夜", "十三夜", "星月夜", "天高し", "秋の空", "鰯雲",
		"秋晴", "秋晴れ",
		// 地理
		"秋の水", "秋の川", "秋の海", "秋の山", "秋の田", "刈田",
		// 生活
		"運動会", "文化祭", "紅葉狩", "新米", "稲刈", "案山子",
		"月見", "秋祭", "菊人形", "七五三",
		// 動物
		"赤蜻蛉", "赤とんぼ", "虫の声", "蟋蟀", "こおろぎ", "コオロギ",
		"鈴虫", "松虫", "蓑虫", "鶴", "鷹", "雁", "渡り鳥",
		"秋刀魚", "さんま",
		// 植物
		"紅葉", "もみじ", "黄葉", "銀杏", "いちょう", "落葉", "枯葉",
		"菊", "コスモス", "秋桜", "萩", "芒", "すすき", "薄",
		"彼岸花", "曼珠沙華", "稲", "実り", "柿", "栗", "葡萄", "林檎",
		"梨", "松茸", "茸", "きのこ", "団栗", "どんぐり", "木の実",
	}

	// ===== 冬 (Winter) =====
	winterKigo := []string{
		// 時候
		"冬", "立冬", "初冬", "冬至", "小寒", "大寒", "寒", "寒し", "寒さ",
		"冴ゆる", "冴える", "冬の朝", "冬の夜", "短日", "冬の暮",
		"年の暮", "年末", "大晦日", "除夜", "師走",
		// 天文
		"雪", "初雪", "吹雪", "粉雪", "深雪", "雪降る", "霰", "あられ",
		"霙", "みぞれ", "氷", "霜", "霜柱", "北風", "木枯し", "木枯らし",
		"空っ風", "冬の月", "冬の星", "オリオン", "冬晴",
		// 地理
		"枯野", "冬の海", "冬の川", "冬の山", "氷柱", "つらら",
		// 生活
		"炬燵", "こたつ", "コタツ", "暖炉", "焚火", "たき火",
		"湯たんぽ", "マフラー", "手袋", "コート", "息白し",
		"鍋", "おでん", "熱燗", "餅", "蜜柑", "みかん",
		"スキー", "スケート", "クリスマス", "年賀状",
		// 動物
		"鶴", "白鳥", "千鳥", "鷲", "鷹", "河豚", "ふぐ", "鱈",
		// 植物
		"枯木", "枯枝", "裸木", "冬木", "冬木立", "枯草",
		"水仙", "山茶花", "さざんか", "寒椿", "寒梅",
		"冬菜", "大根", "白菜", "蕪", "葱",
	}

	// ===== 新年 (New Year) =====
	newYearKigo := []string{
		"正月", "元日", "元旦", "初日の出", "初春", "迎春",
		"初詣", "初夢", "年賀", "お年玉", "門松", "注連飾",
		"鏡餅", "雑煮", "お節", "おせち", "屠蘇", "七草",
		"七草粥", "松の内", "小正月", "成人の日", "書初",
		"初笑い", "福笑い", "凧揚げ", "羽根突き", "独楽",
	}

	for _, w := range springKigo {
		kigoMap[w] = SeasonSpring
	}
	for _, w := range summerKigo {
		kigoMap[w] = SeasonSummer
	}
	for _, w := range autumnKigo {
		kigoMap[w] = SeasonAutumn
	}
	for _, w := range winterKigo {
		kigoMap[w] = SeasonWinter
	}
	for _, w := range newYearKigo {
		kigoMap[w] = SeasonNewYear
	}
}

// detectKigo checks if the given text contains a kigo.
// Returns the longest matched kigo and its season, or nil if none found.
// Longer matches take priority to avoid partial matches (e.g. "初雪" over "雪").
func detectKigo(text string) *KigoResult {
	var best *KigoResult
	bestLen := 0

	for kigo, season := range kigoMap {
		if strings.Contains(text, kigo) {
			if len(kigo) > bestLen {
				bestLen = len(kigo)
				best = &KigoResult{Word: kigo, Season: season}
			}
		}
	}
	return best
}
