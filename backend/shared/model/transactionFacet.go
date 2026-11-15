package model

import "github.com/google/uuid"

// TransactionFacet is one filter value (a payee, account, category or tag) and how many
// matching transactions carry it.
type TransactionFacet struct {
	ID    uuid.UUID `json:"id"`
	Count int       `json:"count"`
}

// TransactionFacets lists the values found in the transactions a filter
// matches, so a filter can be narrowed to what's actually there. Each list
// ignores its own filter: with one payee picked, Payees still shows every
// payee the other filters allow, so more can be added.
type TransactionFacets struct {
	Payees     []TransactionFacet `json:"payees"`
	Accounts   []TransactionFacet `json:"accounts"`
	Categories []TransactionFacet `json:"categories"`
	Tags       []TransactionFacet `json:"tags"`
}
