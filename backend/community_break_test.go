package backend

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type scriptedTransport struct {
	responses []func() *http.Response
	calls     atomic.Int32
}

func (s *scriptedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	index := int(s.calls.Add(1)) - 1
	if index >= len(s.responses) {
		return nil, errors.New("unexpected request")
	}
	return s.responses[index](), nil
}

func scheduledBreakResponse(retryAfter string) func() *http.Response {
	return func() *http.Response {
		header := http.Header{}
		if retryAfter != "" {
			header.Set("Retry-After", retryAfter)
		}
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(`{"error":"The server is taking a scheduled short break."}`)),
		}
	}
}

func okResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"url":"https://example.test/track.flac"}`))}
}

func newCommunityTestRequest() (*http.Request, error) {
	return http.NewRequest(http.MethodPost, "http://community.test/api/dl", strings.NewReader(`{"id":"1"}`))
}

func TestCommunityBreakWaitsAndResumes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, finish := BeginDownloadCancellationScope()
		defer finish()
		ClearCommunityCooldown()

		transport := &scriptedTransport{responses: []func() *http.Response{
			scheduledBreakResponse("600"),
			scheduledBreakResponse("300"),
			okResponse,
		}}
		client := &http.Client{Transport: transport}

		start := time.Now()
		var resp *http.Response
		var err error
		done := make(chan struct{})
		go func() {
			resp, err = doCommunityRequest(client, "Test", newCommunityTestRequest)
			close(done)
		}()

		synctest.Wait()
		progress := GetDownloadProgress()
		if !progress.Cooldown || progress.CooldownSecs < 601 || progress.CooldownSecs > 631 {
			t.Fatalf("first wait: cooldown=%v secs=%d, want active for 601-631s", progress.Cooldown, progress.CooldownSecs)
		}
		firstEventID := progress.CooldownEventID

		time.Sleep(632 * time.Second)
		synctest.Wait()
		progress = GetDownloadProgress()
		if !progress.Cooldown {
			t.Fatal("second wait: cooldown should still be active after another 503")
		}
		if progress.CooldownEventID != firstEventID {
			t.Fatalf("event id changed within one break: %d -> %d", firstEventID, progress.CooldownEventID)
		}

		<-done
		if err != nil {
			t.Fatalf("doCommunityRequest returned error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if calls := transport.calls.Load(); calls != 3 {
			t.Fatalf("requests = %d, want 3", calls)
		}
		if elapsed := time.Since(start); elapsed < 902*time.Second || elapsed > 962*time.Second {
			t.Fatalf("elapsed = %v, want 902-962s (two waits of Retry-After+1s plus up to 30s jitter)", elapsed)
		}
		if GetDownloadProgress().Cooldown {
			t.Fatal("cooldown should be cleared once the server responds normally")
		}
	})
}

func TestCommunityBreakWaitStopsOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, finish := BeginDownloadCancellationScope()
		defer finish()
		ClearCommunityCooldown()

		transport := &scriptedTransport{responses: []func() *http.Response{scheduledBreakResponse("600")}}
		client := &http.Client{Transport: transport}

		errCh := make(chan error, 1)
		go func() {
			_, err := doCommunityRequest(client, "Test", newCommunityTestRequest)
			errCh <- err
		}()

		synctest.Wait()
		start := time.Now()
		ForceStopActiveDownloads()
		err := <-errCh
		if !IsDownloadCancelledError(err) {
			t.Fatalf("err = %v, want download cancelled", err)
		}
		if elapsed := time.Since(start); elapsed != 0 {
			t.Fatalf("stop took %v of waiting, want immediate", elapsed)
		}
		if GetDownloadProgress().Cooldown {
			t.Fatal("cooldown should be cleared when the wait is cancelled")
		}
	})
}

func TestCommunityBreakWithoutDownloadScopeFailsFast(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ClearCommunityCooldown()
		transport := &scriptedTransport{responses: []func() *http.Response{scheduledBreakResponse("")}}
		client := &http.Client{Transport: transport}

		start := time.Now()
		_, err := doCommunityRequest(client, "Test", newCommunityTestRequest)
		if !IsCommunityCooldownError(err) {
			t.Fatalf("err = %v, want community cooldown error", err)
		}
		if elapsed := time.Since(start); elapsed != 0 {
			t.Fatalf("waited %v outside a download, want no wait", elapsed)
		}
		if progress := GetDownloadProgress(); !progress.Cooldown || progress.CooldownSecs != 30 {
			t.Fatalf("cooldown=%v secs=%d, want active with 30s fallback", progress.Cooldown, progress.CooldownSecs)
		}
		ClearCommunityCooldown()
	})
}

func TestCooldownEventIDChangesOnlyForNewBreak(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ClearCommunityCooldown()
		SetCommunityCooldown(60, "break")
		first := GetDownloadProgress().CooldownEventID

		time.Sleep(90 * time.Second)
		SetCommunityCooldown(60, "break extended")
		if id := GetDownloadProgress().CooldownEventID; id != first {
			t.Fatalf("extending a break changed the event id: %d -> %d", first, id)
		}

		time.Sleep(60*time.Second + cooldownEventGap + time.Second)
		SetCommunityCooldown(60, "next break")
		second := GetDownloadProgress().CooldownEventID
		if second == first {
			t.Fatal("a break after the gap should get a new event id")
		}

		time.Sleep(time.Second)
		ClearCommunityCooldown()
		SetCommunityCooldown(60, "after clear")
		if id := GetDownloadProgress().CooldownEventID; id == second {
			t.Fatal("a break after the cooldown was cleared should get a new event id")
		}
		ClearCommunityCooldown()
	})
}
