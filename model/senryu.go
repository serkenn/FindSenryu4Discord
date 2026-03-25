package model

import "time"

// PoemType represents the type of detected poem.
const (
	PoemTypeSenryu  = "senryu"  // 川柳 (5-7-5)
	PoemTypeTanka   = "tanka"   // 短歌 (5-7-5-7-7)
	PoemTypeJiyuritsu = "jiyuritsu" // 自由律俳句 (whitelist match)
)

// Senryu is struct of senryu (also used for tanka and free-form haiku).
type Senryu struct {
	ID        int       `gorm:"primaryKey;autoIncrement"`
	ServerID  string    `gorm:"column:server_id;index"`
	AuthorID  string    `gorm:"column:author_id;index"`
	Kamigo    string    `gorm:"column:kamigo"`
	Nakasichi string    `gorm:"column:nakasichi"`
	Simogo    string    `gorm:"column:simogo"`
	Shiku     string    `gorm:"column:shiku"`              // 四句 (tanka only)
	Goku      string    `gorm:"column:goku"`               // 五句 (tanka only)
	Type      string    `gorm:"column:type;default:senryu"` // senryu, tanka, jiyuritsu
	Spoiler   *bool     `gorm:"column:spoiler;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

// MutedChannel is struct of muted channel.
type MutedChannel struct {
	ChannelID string `gorm:"primaryKey"`
	GuildID   string `gorm:"column:guild_id;index"`
}

// GuildChannelTypeSetting stores per-guild channel type overrides.
// Only rows that differ from the default are stored.
type GuildChannelTypeSetting struct {
	GuildID     string `gorm:"primaryKey;column:guild_id"`
	ChannelType int    `gorm:"primaryKey;column:channel_type"`
	Enabled     bool   `gorm:"column:enabled"`
}

// DetectionOptOut is struct of per-user detection opt-out.
type DetectionOptOut struct {
	ServerID string `gorm:"primaryKey"`
	UserID   string `gorm:"primaryKey"`
}

// BackgroundImage stores custom background image metadata per guild.
type BackgroundImage struct {
	GuildID   string    `gorm:"primaryKey;column:guild_id"`
	FilePath  string    `gorm:"column:file_path"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}
