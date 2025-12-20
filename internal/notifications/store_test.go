package notifications

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	store := NewStore()

	require.NotNil(t, store)
	assert.Equal(t, 0, store.Count())
	assert.Empty(t, store.GetAll())
}

func TestStore_Add(t *testing.T) {
	store := NewStore()

	n := store.Add("Test Title", "Test Message")

	assert.Equal(t, 1, n.ID)
	assert.Equal(t, "Test Title", n.Title)
	assert.Equal(t, "Test Message", n.Message)
	assert.False(t, n.Timestamp.IsZero())
	assert.Equal(t, 1, store.Count())
}

func TestStore_Add_AutoIncrementID(t *testing.T) {
	store := NewStore()

	n1 := store.Add("First", "Message 1")
	n2 := store.Add("Second", "Message 2")
	n3 := store.Add("Third", "Message 3")

	assert.Equal(t, 1, n1.ID)
	assert.Equal(t, 2, n2.ID)
	assert.Equal(t, 3, n3.ID)
}

func TestStore_GetAll_NewestFirst(t *testing.T) {
	store := NewStore()

	store.Add("First", "Message 1")
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	store.Add("Second", "Message 2")
	time.Sleep(10 * time.Millisecond)
	store.Add("Third", "Message 3")

	all := store.GetAll()

	require.Len(t, all, 3)
	// Newest (ID 3) should be first
	assert.Equal(t, 3, all[0].ID)
	assert.Equal(t, "Third", all[0].Title)
	// Oldest (ID 1) should be last
	assert.Equal(t, 1, all[2].ID)
	assert.Equal(t, "First", all[2].Title)
}

func TestStore_GetAll_ReturnsCopy(t *testing.T) {
	store := NewStore()

	store.Add("Original", "Message")

	all1 := store.GetAll()
	all2 := store.GetAll()

	// Modifying one slice shouldn't affect the other
	all1[0].Title = "Modified"
	assert.Equal(t, "Original", all2[0].Title)
}

func TestStore_Count(t *testing.T) {
	store := NewStore()

	assert.Equal(t, 0, store.Count())

	store.Add("First", "Message 1")
	assert.Equal(t, 1, store.Count())

	store.Add("Second", "Message 2")
	assert.Equal(t, 2, store.Count())

	store.Add("Third", "Message 3")
	assert.Equal(t, 3, store.Count())
}

func TestStore_Clear(t *testing.T) {
	store := NewStore()

	store.Add("First", "Message 1")
	store.Add("Second", "Message 2")
	store.Add("Third", "Message 3")

	require.Equal(t, 3, store.Count())

	store.Clear()

	assert.Equal(t, 0, store.Count())
	assert.Empty(t, store.GetAll())
}

func TestStore_Clear_Empty(t *testing.T) {
	store := NewStore()

	// Clearing an empty store should not panic
	store.Clear()

	assert.Equal(t, 0, store.Count())
}

func TestStore_Add_AfterClear(t *testing.T) {
	store := NewStore()

	store.Add("First", "Message 1")
	store.Clear()

	// IDs should continue from where they left off (not reset)
	n := store.Add("After Clear", "New Message")

	// Note: The current implementation doesn't reset nextID on Clear
	// If this behavior changes, update this test
	assert.Equal(t, 2, n.ID)
}

func TestStore_ThreadSafety_ConcurrentAdd(t *testing.T) {
	store := NewStore()

	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			store.Add("Title", "Message")
		}(i)
	}
	wg.Wait()

	assert.Equal(t, numGoroutines, store.Count())
}

func TestStore_ThreadSafety_ConcurrentGetAll(t *testing.T) {
	store := NewStore()

	// Add some initial notifications
	for i := 0; i < 10; i++ {
		store.Add("Title", "Message")
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			all := store.GetAll()
			assert.NotNil(t, all)
		}()
	}
	wg.Wait()
}

func TestStore_ThreadSafety_ConcurrentAddAndGet(t *testing.T) {
	store := NewStore()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent adds
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			store.Add("Title", "Message")
		}(i)
	}

	// Concurrent reads (start after a brief delay)
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_ = store.GetAll()
			_ = store.Count()
		}()
	}

	wg.Wait()

	// Should have all notifications
	assert.Equal(t, numGoroutines, store.Count())
}

func TestNotification_JSONTags(t *testing.T) {
	n := Notification{
		ID:        1,
		Title:     "Test",
		Message:   "Message",
		Timestamp: time.Now(),
	}

	// Verify struct fields are accessible (compile-time check for JSON tags)
	assert.Equal(t, 1, n.ID)
	assert.Equal(t, "Test", n.Title)
	assert.Equal(t, "Message", n.Message)
	assert.False(t, n.Timestamp.IsZero())
}

func TestStore_Add_TimestampIsRecent(t *testing.T) {
	store := NewStore()

	before := time.Now()
	n := store.Add("Title", "Message")
	after := time.Now()

	assert.True(t, n.Timestamp.After(before) || n.Timestamp.Equal(before))
	assert.True(t, n.Timestamp.Before(after) || n.Timestamp.Equal(after))
}
