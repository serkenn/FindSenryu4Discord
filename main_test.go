package main

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	"github.com/u16-io/FindSenryu4Discord/db"
	"github.com/u16-io/FindSenryu4Discord/model"
	"github.com/u16-io/FindSenryu4Discord/service"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	var err error
	db.DB, err = gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.DB.AutoMigrate(&model.MutedChannel{}, &model.GuildChannelTypeSetting{}).Error; err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	t.Cleanup(func() {
		db.DB.Close()
	})
}

func TestIsChannelTypeEnabled_デフォルト有効タイプ(t *testing.T) {
	setupTestDB(t)

	enabledTypes := []discordgo.ChannelType{
		discordgo.ChannelTypeGuildText,
		discordgo.ChannelTypeGuildVoice,
		discordgo.ChannelTypeGuildStageVoice,
		discordgo.ChannelTypeGuildNewsThread,
		discordgo.ChannelTypeGuildPublicThread,
		discordgo.ChannelTypeGuildPrivateThread,
	}

	for _, ct := range enabledTypes {
		if !service.IsChannelTypeEnabled("test-guild", ct) {
			t.Errorf("channel type %d should be enabled by default", ct)
		}
	}
}

func TestIsChannelTypeEnabled_デフォルト無効タイプ(t *testing.T) {
	setupTestDB(t)

	disabledTypes := []discordgo.ChannelType{
		discordgo.ChannelTypeGuildNews,
		discordgo.ChannelTypeGuildForum,
	}

	for _, ct := range disabledTypes {
		if service.IsChannelTypeEnabled("test-guild", ct) {
			t.Errorf("channel type %d should be disabled by default", ct)
		}
	}
}

func TestIsChannelTypeEnabled_未知のタイプは無効(t *testing.T) {
	setupTestDB(t)

	if service.IsChannelTypeEnabled("test-guild", discordgo.ChannelType(999)) {
		t.Error("unknown channel type should be disabled")
	}
}

func TestContainsDiscordTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"ユーザーメンション", "<@123456789> こんにちは", true},
		{"ニックネーム付きメンション", "<@!123456789> こんにちは", true},
		{"チャンネルメンション", "<#987654321> で話しましょう", true},
		{"ロールメンション", "<@&111222333> に連絡", true},
		{"カスタム絵文字", "すごい <:emoji:123456> ですね", true},
		{"アニメーション絵文字", "楽しい <a:dance:789012> 時間", true},
		{"URL_https", "詳細は https://example.com を参照", true},
		{"URL_http", "リンク http://example.com です", true},
		{"トークンなし", "古池や蛙飛び込む水の音", false},
		{"空文字列", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsDiscordTokens(tt.input)
			if got != tt.want {
				t.Errorf("containsDiscordTokens(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestContainsSpoiler(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"スポイラーあり", "これは||ネタバレ||です", true},
		{"スポイラーなし", "古池や蛙飛び込む水の音", false},
		{"複数スポイラー", "||秘密||と||内緒||の話", true},
		{"パイプ1本", "条件A|条件B", false},
		{"空文字列", "", false},
		{"スポイラー内が空", "||||", false},
		{"スポイラー内にスペース", "||秘密の 内容||です", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsSpoiler(tt.input)
			if got != tt.want {
				t.Errorf("containsSpoiler(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripSpoilerMarkers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"スポイラーあり", "これは||ネタバレ||です", "これはネタバレです"},
		{"スポイラーなし", "普通のテキスト", "普通のテキスト"},
		{"複数スポイラー", "||秘密||と||内緒||の話", "秘密と内緒の話"},
		{"空文字列", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripSpoilerMarkers(tt.input)
			if got != tt.want {
				t.Errorf("stripSpoilerMarkers(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsParentChannelMuted_親チャンネルがミュート(t *testing.T) {
	setupTestDB(t)

	if err := service.ToMute("parent-channel", "test-guild"); err != nil {
		t.Fatalf("failed to mute parent channel: %v", err)
	}

	ch := &discordgo.Channel{ParentID: "parent-channel"}
	if !isParentChannelMuted(ch) {
		t.Error("should detect parent channel as muted")
	}
}

func TestIsParentChannelMuted_親チャンネルがミュートされていない(t *testing.T) {
	setupTestDB(t)

	ch := &discordgo.Channel{ParentID: "unmuted-parent"}
	if isParentChannelMuted(ch) {
		t.Error("should not detect unmuted parent channel as muted")
	}
}

func TestIsParentChannelMuted_親チャンネルなし(t *testing.T) {
	setupTestDB(t)

	ch := &discordgo.Channel{ParentID: ""}
	if isParentChannelMuted(ch) {
		t.Error("channel with no parent should not be considered muted")
	}
}

func TestIsParentChannelMuted_自チャンネルのミュートは親に影響しない(t *testing.T) {
	setupTestDB(t)

	if err := service.ToMute("thread-channel", "test-guild"); err != nil {
		t.Fatalf("failed to mute thread channel: %v", err)
	}

	ch := &discordgo.Channel{
		ID:       "thread-channel",
		ParentID: "other-parent",
	}
	if isParentChannelMuted(ch) {
		t.Error("muting the thread itself should not affect parent mute check")
	}
}

// --- False-positive prevention filter tests ---

func TestJapaneseRatio(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMin float64
		wantMax float64
	}{
		{"純粋な日本語", "古池や蛙飛び込む水の音", 1.0, 1.0},
		{"ASCII混在", "hello世界", 0.2, 0.4},
		{"全ASCII", "hello world", 0.0, 0.0},
		{"空文字", "", 0.0, 0.0},
		{"漢字のみ", "国破山河在", 1.0, 1.0},
		{"混在（高比率）", "今日はいい天気ですね", 0.9, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := japaneseRatio(tt.input)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("japaneseRatio(%q) = %f, want [%f, %f]", tt.input, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestHasMiddleSentencePunctuation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"文中に句点", "それはいいね。でも明日にしよう", true},
		{"文中に？", "本当？まじで", true},
		{"文中に！", "すごい！やったね", true},
		{"末尾のみ句点", "それはいいね。", false},
		{"末尾のみ？", "本当？", false},
		{"句読点なし", "古池や蛙飛び込む水の音", false},
		{"空文字", "", false},
		{"1文字", "あ", false},
		{"半角!文中", "wow!すごい", true},
		{"末尾のみ半角!", "すごい!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMiddleSentencePunctuation(tt.input)
			if got != tt.want {
				t.Errorf("hasMiddleSentencePunctuation(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsFalsePositivePhrase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"おはようございます", "おはようございます", true},
		{"ありがとうございます含む", "本当にありがとうございます", true},
		{"川柳", "古池や蛙飛び込む水の音", false},
		{"空文字", "", false},
		{"おやすみなさい", "おやすみなさい", true},
		{"普通の文", "明日は雨が降るらしい", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFalsePositivePhrase(tt.input)
			if got != tt.want {
				t.Errorf("isFalsePositivePhrase(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsJapaneseChar(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{"ひらがな", 'あ', true},
		{"カタカナ", 'ア', true},
		{"漢字", '山', true},
		{"ASCII", 'a', false},
		{"数字", '1', false},
		{"全角数字", '１', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isJapaneseChar(tt.r)
			if got != tt.want {
				t.Errorf("isJapaneseChar(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}
