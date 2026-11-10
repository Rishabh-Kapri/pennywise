package websocket

import (
	"testing"
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func TestDeliverSurvivesAClosedClient(t *testing.T) {
	c := &Client{Send: make(chan sharedModel.Message)}
	close(c.Send)
	deliver(c, sharedModel.Message{}, 0)
}

func TestDeliverGivesUpOnAStuckClient(t *testing.T) {
	c := &Client{Send: make(chan sharedModel.Message)} // nothing reading
	done := make(chan struct{})
	go func() {
		deliver(c, sharedModel.Message{}, 10*time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("deliver blocked on a client that isn't reading")
	}
}
