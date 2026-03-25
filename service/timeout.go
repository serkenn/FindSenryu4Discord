package service

import (
	"sync"
	"time"
)

// timeoutMap stores user timeouts: key = "channelID:userID", value = expiry time.
var (
	timeoutMap   = make(map[string]time.Time)
	timeoutMutex sync.RWMutex
)

func timeoutKey(channelID, userID string) string {
	return channelID + ":" + userID
}

// SetTimeout sets a temporary detection timeout for a user in a channel.
func SetTimeout(channelID, userID string, duration time.Duration) {
	timeoutMutex.Lock()
	defer timeoutMutex.Unlock()
	timeoutMap[timeoutKey(channelID, userID)] = time.Now().Add(duration)
}

// ClearTimeout removes a timeout for a user in a channel.
func ClearTimeout(channelID, userID string) {
	timeoutMutex.Lock()
	defer timeoutMutex.Unlock()
	delete(timeoutMap, timeoutKey(channelID, userID))
}

// IsTimedOut checks if a user currently has an active timeout in a channel.
func IsTimedOut(channelID, userID string) bool {
	timeoutMutex.RLock()
	defer timeoutMutex.RUnlock()

	expiry, ok := timeoutMap[timeoutKey(channelID, userID)]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		return false
	}
	return true
}

// GetTimeoutRemaining returns the remaining duration of a user's timeout in a channel.
// Returns 0 if no active timeout.
func GetTimeoutRemaining(channelID, userID string) time.Duration {
	timeoutMutex.RLock()
	defer timeoutMutex.RUnlock()

	expiry, ok := timeoutMap[timeoutKey(channelID, userID)]
	if !ok {
		return 0
	}
	remaining := time.Until(expiry)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CleanupExpiredTimeouts removes expired entries from the timeout map.
func CleanupExpiredTimeouts() {
	timeoutMutex.Lock()
	defer timeoutMutex.Unlock()

	now := time.Now()
	for k, expiry := range timeoutMap {
		if now.After(expiry) {
			delete(timeoutMap, k)
		}
	}
}
