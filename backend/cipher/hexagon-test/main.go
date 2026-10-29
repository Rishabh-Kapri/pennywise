// Scratch file for trying the regex adapter from outside its package.
// Run from backend/cipher: go run ./hexagon-test
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/noop"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/regex"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
)

func configureAdapter(name string) (ports.CheckTransaction, error) {
	// suppose we have "regex" as the adapter for CheckTransactionPort defined
	switch name {
	case "regex":
		return regex.NewCheckTransaction(), nil
	case "noop":
		return noop.NewCheckTransaction(), nil
	default:
		return nil, fmt.Errorf("adapter \"%s\" for CheckTransaction not configured", name)
	}
}

func main() {
	detector, err := configureAdapter("llm")

	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	msg := ports.Message{
		Body:   "Spent Rs.3782 On HDFC Bank Card 2222 At CONFIRM12344135 On 2026-10-28:21:19:35.",
		Source: ports.SMSSource,
	}

	verdict, err := detector.IsTransaction(context.Background(), msg)
	fmt.Printf("type: %T\nverdict: %+v\nerr: %v\n", detector, verdict, err)
}
