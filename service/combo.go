package service

import (
	"fmt"
	"sync"
	"time"
)

// comboEntry tracks consecutive poem detections.
type comboEntry struct {
	Count     int
	LastTime  time.Time
}

const comboTimeout = 5 * time.Minute // combo resets if no detection within this period

var (
	// serverCombo tracks consecutive detections per server (guild).
	serverCombo   = make(map[string]*comboEntry)
	serverComboMu sync.RWMutex

	// userCombo tracks consecutive detections per user per server.
	// Key: "guildID:userID"
	userCombo   = make(map[string]*comboEntry)
	userComboMu sync.RWMutex
)

// ComboResult holds the combo counts after recording a detection.
type ComboResult struct {
	ServerCombo int // total consecutive detections in the server
	UserCombo   int // consecutive detections by this user in this server
}

// RecordCombo records a poem detection and returns the updated combo counts.
func RecordCombo(guildID, userID string) ComboResult {
	now := time.Now()

	// Server combo
	serverComboMu.Lock()
	sc := serverCombo[guildID]
	if sc == nil || now.Sub(sc.LastTime) > comboTimeout {
		sc = &comboEntry{}
		serverCombo[guildID] = sc
	}
	sc.Count++
	sc.LastTime = now
	serverCount := sc.Count
	serverComboMu.Unlock()

	// User combo
	key := guildID + ":" + userID
	userComboMu.Lock()
	uc := userCombo[key]
	if uc == nil || now.Sub(uc.LastTime) > comboTimeout {
		uc = &comboEntry{}
		userCombo[key] = uc
	}
	uc.Count++
	uc.LastTime = now
	userCount := uc.Count
	userComboMu.Unlock()

	return ComboResult{
		ServerCombo: serverCount,
		UserCombo:   userCount,
	}
}

// GetComboText returns a combo announcement string, or empty if no combo.
func GetComboText(combo ComboResult) string {
	var parts []string

	if combo.ServerCombo >= 2 {
		parts = append(parts, fmt.Sprintf("サーバー %d連鎖！", combo.ServerCombo))
	}
	if combo.UserCombo >= 2 {
		parts = append(parts, fmt.Sprintf("個人 %d連鎖！", combo.UserCombo))
	}

	if len(parts) == 0 {
		return ""
	}

	text := "🔥 "
	for i, p := range parts {
		if i > 0 {
			text += " / "
		}
		text += p
	}

	// Add escalating emoji for higher combos
	maxCombo := combo.ServerCombo
	if combo.UserCombo > maxCombo {
		maxCombo = combo.UserCombo
	}
	switch {
	case maxCombo >= 10:
		text += " 🌋🌋🌋"
	case maxCombo >= 7:
		text += " 🔥🔥"
	case maxCombo >= 5:
		text += " ✨🔥"
	case maxCombo >= 3:
		text += " ✨"
	}

	return text
}

// CleanupExpiredCombos removes stale combo entries to prevent memory leaks.
func CleanupExpiredCombos() {
	now := time.Now()

	serverComboMu.Lock()
	for k, v := range serverCombo {
		if now.Sub(v.LastTime) > comboTimeout*2 {
			delete(serverCombo, k)
		}
	}
	serverComboMu.Unlock()

	userComboMu.Lock()
	for k, v := range userCombo {
		if now.Sub(v.LastTime) > comboTimeout*2 {
			delete(userCombo, k)
		}
	}
	userComboMu.Unlock()
}
