package jobs

import "testing"

func TestCanTransition(t *testing.T) {
	legal := []struct{ from, to Status }{
		{StatusQueued, StatusRunning},
		{StatusQueued, StatusCancelled},
		{StatusRunning, StatusVerifying},
		{StatusRunning, StatusFailed},
		{StatusRunning, StatusCancelled},
		{StatusVerifying, StatusCompleted},
		{StatusVerifying, StatusFailed},
		{StatusVerifying, StatusCancelled},
	}
	for _, c := range legal {
		if !CanTransition(c.from, c.to) {
			t.Errorf("expected %s→%s to be legal", c.from, c.to)
		}
	}

	illegal := []struct{ from, to Status }{
		{StatusQueued, StatusVerifying},
		{StatusQueued, StatusCompleted},
		{StatusRunning, StatusCompleted},
		{StatusRunning, StatusQueued},
		{StatusCompleted, StatusRunning},
		{StatusFailed, StatusRunning},
		{StatusCancelled, StatusQueued},
		{StatusVerifying, StatusRunning},
	}
	for _, c := range illegal {
		if CanTransition(c.from, c.to) {
			t.Errorf("expected %s→%s to be illegal", c.from, c.to)
		}
	}
}
