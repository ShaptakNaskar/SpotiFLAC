package backend

import (
	"context"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestWatchDownloadProgressEmitsOnlyChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		SetDownloading(false)
		ClearCommunityCooldown()
		defer func() {
			SetDownloading(false)
			ClearCommunityCooldown()
		}()

		var mu sync.Mutex
		var updates []ProgressInfo
		received := func() []ProgressInfo {
			synctest.Wait()
			mu.Lock()
			defer mu.Unlock()
			return slices.Clone(updates)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go WatchDownloadProgress(ctx, 200*time.Millisecond, func(progress ProgressInfo) {
			mu.Lock()
			updates = append(updates, progress)
			mu.Unlock()
		})

		time.Sleep(time.Second)
		if got := received(); len(got) != 0 {
			t.Fatalf("idle app emitted %d updates, want none", len(got))
		}

		SetDownloading(true)
		SetDownloadProgress(1.5)
		time.Sleep(250 * time.Millisecond)
		got := received()
		if len(got) != 1 || !got[0].IsDownloading || got[0].MBDownloaded != 1.5 {
			t.Fatalf("after a progress change got %+v, want one downloading update at 1.5 MB", got)
		}

		time.Sleep(time.Second)
		if got := received(); len(got) != 1 {
			t.Fatalf("unchanged progress was emitted again: %d updates", len(got))
		}

		SetCommunityCooldown(3, "break")
		time.Sleep(3500 * time.Millisecond)
		countdown := received()[1:]
		var secs []int
		for _, update := range countdown {
			if update.Cooldown {
				secs = append(secs, update.CooldownSecs)
			}
		}
		if len(countdown) == 0 || !slices.Equal(secs, []int{3, 2, 1}) || countdown[len(countdown)-1].Cooldown {
			t.Fatalf("break countdown updates = %v from %d updates, want [3 2 1] followed by one that ends the break", secs, len(countdown))
		}
	})
}
