package commands

import (
	"github.com/bwmarrin/discordgo"
	"github.com/u16-io/FindSenryu4Discord/pkg/metrics"
)

// HandleHelpCommand handles the /help slash command.
func HandleHelpCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("help")

	embed := &discordgo.MessageEmbed{
		Title: "📖 俳句・川柳・短歌のルール",
		Color: 0x8B4513,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "🔹 俳句・川柳（5-7-5）",
				Value: "**上の句（5音）＋ 中の句（7音）＋ 下の句（5音）**\n" +
					"例：「古池や　蛙飛び込む　水の音」\n" +
					"＊俳句は季語を含み、川柳は季語不要で人事を詠む",
			},
			{
				Name: "🔹 短歌（5-7-5-7-7）",
				Value: "**上の句（5-7-5）＋ 下の句（7-7）**\n" +
					"例：「田子の浦に　うち出でてみれば　白妙の　富士の高嶺に　雪は降りつつ」",
			},
			{
				Name: "🔹 五言律詩（5×8）",
				Value: "**漢字5文字 × 8句の漢詩**\n" +
					"例：「国破山河在　城春草木深…」",
			},
			{
				Name: "🔹 自由律俳句",
				Value: "定型に囚われない俳句。有名句がホワイトリストで登録されています\n" +
					"例：「咳をしても一人」（尾崎放哉）",
			},
			{
				Name: "📐 音の数え方",
				Value: "```\n" +
					"■ 基本：仮名1文字 = 1音\n" +
					"  「さくら」= 3音\n\n" +
					"■ 拗音（ゃ・ゅ・ょ）= 前の文字と合わせて1音\n" +
					"  「きょ」= 1音、「しゃ」= 1音\n\n" +
					"■ 促音（っ）= 1音\n" +
					"  「きって」= 3音（き・っ・て）\n\n" +
					"■ 撥音（ん）= 1音\n" +
					"  「さんま」= 3音（さ・ん・ま）\n\n" +
					"■ 長音（ー）= 1音\n" +
					"  「ケーキ」= 3音（ケ・ー・キ）\n" +
					"```",
			},
			{
				Name: "⚙️ コマンド一覧",
				Value: "`/detect on|off|status` — 川柳検出のオン/オフ\n" +
					"`/blacklist` — 検出トグル\n" +
					"`/timeout <分>` — 一時停止（管理者/許可ロール）\n" +
					"`/compose` — 手動で川柳を作成\n" +
					"`/rank` — ランキング表示\n" +
					"`/mute` / `/unmute` — チャンネルミュート\n" +
					"`/channel` — チャンネルタイプ別設定\n" +
					"`/timeout-role add|remove|list` — timeout権限ロール管理\n" +
					"`/doctor` — 診断\n" +
					"`/contact` — 管理者に連絡",
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "川柳 日本の心Bot | メッセージを送るだけで自動検出！",
		},
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}
