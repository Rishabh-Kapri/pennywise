package factory

import (
	"context"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
)

type fakeLink struct {
	verdict ports.TxnVerdict
	err     error
	calls   *int
}

func (f fakeLink) Check(context.Context, ports.Message) (ports.TxnVerdict, error) {
	*f.calls++
	return f.verdict, f.err
}

func links(ports_ ...ports.CheckTransaction) []chainLink[ports.CheckTransaction] {
	out := make([]chainLink[ports.CheckTransaction], 0, len(ports_))
	for _, port := range ports_ {
		out = append(out, chainLink[ports.CheckTransaction]{name: "test", port: port})
	}
	return out
}

func TestCheckChain(t *testing.T) {
	boom := errors.New("boom")
	yes := ports.TxnVerdict{IsTransaction: true}

	t.Run("moves to the next model when one fails", func(t *testing.T) {
		first, second := 0, 0
		chain := checkChain{links(
			fakeLink{err: boom, calls: &first},
			fakeLink{verdict: yes, calls: &second},
		)}
		got, err := chain.Check(context.Background(), ports.Message{})
		if err != nil || !got.IsTransaction || first != 1 || second != 1 {
			t.Fatalf("got %+v, %v, calls %d/%d", got, err, first, second)
		}
	})

	t.Run("a no answer is a success and stops the chain", func(t *testing.T) {
		first, second := 0, 0
		chain := checkChain{links(
			fakeLink{calls: &first},
			fakeLink{verdict: yes, calls: &second},
		)}
		got, err := chain.Check(context.Background(), ports.Message{})
		if err != nil || got.IsTransaction || second != 0 {
			t.Fatalf("got %+v, %v, second called %d times", got, err, second)
		}
	})

	t.Run("fails when every model fails", func(t *testing.T) {
		calls := 0
		chain := checkChain{links(
			fakeLink{err: errors.New("first"), calls: &calls},
			fakeLink{err: boom, calls: &calls},
		)}
		if _, err := chain.Check(context.Background(), ports.Message{}); !errors.Is(err, boom) || calls != 2 {
			t.Fatalf("err = %v, calls = %d", err, calls)
		}
	})

	t.Run("stops when the job is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		first, second := 0, 0
		chain := checkChain{links(
			fakeLink{err: boom, calls: &first},
			fakeLink{verdict: yes, calls: &second},
		)}
		if _, err := chain.Check(ctx, ports.Message{}); !errors.Is(err, context.Canceled) || second != 0 {
			t.Fatalf("err = %v, second called %d times", err, second)
		}
	})

	t.Run("no links is an error, not a blank success", func(t *testing.T) {
		if _, err := (checkChain{}).Check(context.Background(), ports.Message{}); err == nil {
			t.Fatal("want an error")
		}
	})
}
