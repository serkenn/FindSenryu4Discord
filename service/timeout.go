package service

import (
	"sync"
	"time"
)

// timeoutMap stores user timeouts: key = "serverID:userID", value = expiry time.
var (
	timeoutMap   = make(map[string]time.Time)
	timeoutMutex sync.RWMutex
)

func timeoutKey(serverID, userID string) string {
	return serverID + ":" + userID
}

// SetTimeout sets a temporary detection timeout for a user in a server.
func SetTimeout(serverID, userID string, duration time.Duration) {
	timeoutMutex.Lock()
	defer timeoutMutex.Unlock()
	timeoutMap[timeoutKey(serverID, userID)] = time.Now().Add(duration)
}

// ClearTimeout removes a timeout for a user.
func ClearTimeout(serverID, userID string) {
	timeoutMutex.Lock()
	defer timeoutMutex.Unlock()
	delete(timeoutMap, timeoutKey(serverID, userID))
}

// IsTimedOut checks if a user currently has an active timeout.
func IsTimedOut(serverID, userID string) bool {
	timeoutMutex.RLock()
	defer timeoutMutex.RUnlock()

	expiry, ok := timeoutMap[timeoutKey(serverID, userID)]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		// Expired — clean up lazily on next write
		return false
	}
	return true
}

// GetTimeoutRemaining returns the remaining duration of a user's timeout.
// Returns 0 if no active timeout.
func GetTimeoutRemaining(serverID, userID string) time.Duration {
	timeoutMutex.RLock()
	defer timeoutMutex.RUnlock()

	expiry, ok := timeoutMap[timeoutKey(serverID, userID)]
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
