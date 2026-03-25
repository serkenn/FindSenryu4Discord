package commands

import (
	"bytes"
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/u16-io/FindSenryu4Discord/model"
	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
	"github.com/u16-io/FindSenryu4Discord/pkg/metrics"
	"github.com/u16-io/FindSenryu4Discord/pkg/senryuimg"
	"github.com/u16-io/FindSenryu4Discord/service"
)

// HandleComposeCommand handles the /compose slash command.
// Creates a senryu with specified phrases attributed to a user, with image.
func HandleComposeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	metrics.RecordCommandExecuted("compose")

	if i.GuildID == "" {
		respondError(s, i, "このコマンドはサーバー内でのみ使用できます")
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) < 3 {
		respondEphemeral(s, i, "上の句・中の句・下の句を指定してください")
		return
	}

	kamigo := options[0].StringValue()
	nakasichi := options[1].StringValue()
	simogo := options[2].StringValue()

	// Determine the target user (default = command invoker)
	callerID := getUserID(i)
	targetUser := i.Member.User
	if len(options) >= 4 && options[3].UserValue(s) != nil {
		targetUser = options[3].UserValue(s)
	}

	// Get display name
	displayName := getComposeDisplayName(s, i.GuildID, targetUser)
	avatarURL := targetUser.AvatarURL("128")

	// Defer the response since image generation may take a moment
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	// Save to DB
	spoiler := false
	_, err := service.CreateSenryu(model.Senryu{
		ServerID:  i.GuildID,
		AuthorID:  targetUser.ID,
		Kamigo:    kamigo,
		Nakasichi: nakasichi,
		Simogo:    simogo,
		Spoiler:   &spoiler,
	})
	if err != nil {
		logger.Error("Failed to create composed senryu", "error", err)
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "川柳の保存に失敗しました",
		})
		return
	}

	// Load custom background
	var bgData []byte
	if bg, bgErr := service.GetBackground(i.GuildID); bgErr == nil && bg != nil {
		if data, readErr := os.ReadFile(bg.FilePath); readErr == nil {
			bgData = data
		}
	}

	// Generate image
	imgData, imgErr := senryuimg.RenderSenryu(senryuimg.RenderOptions{
		Kamigo:     kamigo,
		Nakasichi:  nakasichi,
		Simogo:     simogo,
		AuthorName: displayName,
		AvatarURL:  avatarURL,
		Background: bgData,
	})

	content := fmt.Sprintf("「%s %s %s」\n詠み手: %s", kamigo, nakasichi, simogo, displayName)

	if callerID != targetUser.ID {
		content += fmt.Sprintf("\n作成者: <@%s>", callerID)
	}

	params := &discordgo.WebhookParams{
		Content: content,
	}

	if imgErr != nil {
		logger.Warn("Failed to render composed senryu image", "error", imgErr)
	} else {
		params.Files = []*discordgo.File{{
			Name:        "senryu.webp",
			ContentType: "image/webp",
			Reader:      bytes.NewReader(imgData),
		}}
	}

	if _, err := s.FollowupMessageCreate(i.Interaction, true, params); err != nil {
		logger.Error("Failed to send composed senryu", "error", err)
	}
}

func getComposeDisplayName(s *discordgo.Session, guildID string, user *discordgo.User) string {
	member, err := s.GuildMember(guildID, user.ID)
	if err == nil && member.Nick != "" {
		return member.Nick
	}
	if user.GlobalName != "" {
		return user.GlobalName
	}
	return user.Username
}
