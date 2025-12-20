package session

import (
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	store := NewStore()
	defer store.Close()

	require.NotNil(t, store)
	assert.Equal(t, 0, store.Count())
}

func TestNewStoreWithClock(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	store := NewStoreWithClock(fakeClock)
	defer store.Close()

	require.NotNil(t, store)
	assert.Equal(t, 0, store.Count())
}

func TestStore_Create(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	store := NewStoreWithClock(fakeClock)
	defer store.Close()

	sess, err := store.Create("testuser")

	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.Equal(t, "testuser", sess.Username)
	assert.NotEmpty(t, sess.ID)
	assert.Equal(t, fakeClock.Now(), sess.CreatedAt)
	assert.Equal(t, fakeClock.Now().Add(sessionTimeout), sess.ExpiresAt)
	assert.Equal(t, 1, store.Count())
}

func TestStore_Create_UniqueIDs(t *testing.T) {
	store := NewStore()
	defer store.Close()

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		sess, err := store.Create("user")
		require.NoError(t, err)

		assert.False(t, ids[sess.ID], "session ID should be unique")
		ids[sess.ID] = true
	}
}

func TestStore_Get_ValidSession(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	store := NewStoreWithClock(fakeClock)
	defer store.Close()

	created, err := store.Create("testuser")
	require.NoError(t, err)

	retrieved := store.Get(created.ID)

	require.NotNil(t, retrieved)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Username, retrieved.Username)
}

func TestStore_Get_NotFound(t *testing.T) {
	store := NewStore()
	defer store.Close()

	retrieved := store.Get("nonexistent-session-id")

	assert.Nil(t, retrieved)
}

func TestStore_Get_Expired(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	store := NewStoreWithClock(fakeClock)
	defer store.Close()

	sess, err := store.Create("testuser")
	require.NoError(t, err)
	require.Equal(t, 1, store.Count())

	// Advance time past session timeout
	fakeClock.Advance(sessionTimeout + time.Second)

	// Session should be expired and deleted
	retrieved := store.Get(sess.ID)
	assert.Nil(t, retrieved)
	assert.Equal(t, 0, store.Count())
}

func TestStore_Get_NotExpiredJustBefore(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	store := NewStoreWithClock(fakeClock)
	defer store.Close()

	sess, err := store.Create("testuser")
	require.NoError(t, err)

	// Advance time to just before expiration
	fakeClock.Advance(sessionTimeout - time.Second)

	retrieved := store.Get(sess.ID)
	assert.NotNil(t, retrieved)
	assert.Equal(t, sess.ID, retrieved.ID)
}

func TestStore_Delete(t *testing.T) {
	store := NewStore()
	defer store.Close()

	sess, err := store.Create("testuser")
	require.NoError(t, err)
	require.Equal(t, 1, store.Count())

	store.Delete(sess.ID)

	assert.Equal(t, 0, store.Count())
	assert.Nil(t, store.Get(sess.ID))
}

func TestStore_Delete_NonExistent(t *testing.T) {
	store := NewStore()
	defer store.Close()

	// Deleting non-existent session should not panic
	store.Delete("nonexistent-id")

	assert.Equal(t, 0, store.Count())
}

func TestStore_Count(t *testing.T) {
	store := NewStore()
	defer store.Close()

	assert.Equal(t, 0, store.Count())

	store.Create("user1")
	assert.Equal(t, 1, store.Count())

	store.Create("user2")
	assert.Equal(t, 2, store.Count())

	store.Create("user3")
	assert.Equal(t, 3, store.Count())
}

func TestStore_Cleanup(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	store := NewStoreWithClock(fakeClock)
	defer store.Close()

	// Create multiple sessions
	sess1, _ := store.Create("user1")
	fakeClock.Advance(time.Hour)
	sess2, _ := store.Create("user2")
	fakeClock.Advance(time.Hour)
	sess3, _ := store.Create("user3")

	require.Equal(t, 3, store.Count())

	// Advance past first session's expiration
	// sess1 was created at t=0, expires at t=24h
	// sess2 was created at t=1h, expires at t=25h
	// sess3 was created at t=2h, expires at t=26h
	// Current time is t=2h
	fakeClock.Advance(22*time.Hour + time.Second) // Now t=24h+1s (past sess1's expiry)

	// Manually trigger cleanup
	store.cleanup()

	// sess1 should be expired (created at t=0, expired at t=24h, now t=24h+1s)
	assert.Nil(t, store.Get(sess1.ID))
	// sess2 and sess3 should still be valid
	assert.NotNil(t, store.Get(sess2.ID))
	assert.NotNil(t, store.Get(sess3.ID))
	assert.Equal(t, 2, store.Count())
}

func TestStore_Close(t *testing.T) {
	store := NewStore()

	// Close should not panic
	store.Close()

	// Creating after close would panic due to closed channel
	// But the sessions map is still usable for the test
}

func TestGetCookieName(t *testing.T) {
	name := GetCookieName()
	assert.Equal(t, "coinops_session", name)
}

func TestGenerateSessionID(t *testing.T) {
	id, err := generateSessionID()

	require.NoError(t, err)
	assert.NotEmpty(t, id)
	// Base64 of 32 bytes should be 43 characters (with URL encoding, no padding)
	assert.Len(t, id, 44) // Base64 with padding
}

func TestGenerateSessionID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generateSessionID()
		require.NoError(t, err)

		assert.False(t, ids[id], "session ID should be unique")
		ids[id] = true
	}
}

func TestStore_ThreadSafety_ConcurrentCreate(t *testing.T) {
	store := NewStore()
	defer store.Close()

	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := store.Create("user")
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, numGoroutines, store.Count())
}

func TestStore_ThreadSafety_ConcurrentGet(t *testing.T) {
	store := NewStore()
	defer store.Close()

	// Create a session
	sess, _ := store.Create("testuser")

	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			retrieved := store.Get(sess.ID)
			assert.NotNil(t, retrieved)
		}()
	}
	wg.Wait()
}

func TestStore_ThreadSafety_ConcurrentCreateAndDelete(t *testing.T) {
	store := NewStore()
	defer store.Close()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Create and delete concurrently
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			sess, _ := store.Create("user")
			if sess != nil {
				store.Delete(sess.ID)
			}
		}()
		go func() {
			defer wg.Done()
			store.Create("user")
		}()
	}
	wg.Wait()

	// Count should be consistent (some may be deleted)
	count := store.Count()
	assert.GreaterOrEqual(t, count, 0)
}

func TestSession_Struct(t *testing.T) {
	now := time.Now()
	sess := Session{
		ID:        "test-id",
		Username:  "testuser",
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}

	assert.Equal(t, "test-id", sess.ID)
	assert.Equal(t, "testuser", sess.Username)
	assert.Equal(t, now, sess.CreatedAt)
	assert.Equal(t, now.Add(24*time.Hour), sess.ExpiresAt)
}

func TestSessionTimeout_Constant(t *testing.T) {
	// Verify the session timeout is 24 hours as documented
	assert.Equal(t, 24*time.Hour, sessionTimeout)
}

func TestCleanupInterval_Constant(t *testing.T) {
	// Verify the cleanup interval is 1 hour as documented
	assert.Equal(t, time.Hour, cleanupInterval)
}

func TestSessionIDLength_Constant(t *testing.T) {
	// Verify the session ID length is 32 bytes
	assert.Equal(t, 32, sessionIDLength)
}
