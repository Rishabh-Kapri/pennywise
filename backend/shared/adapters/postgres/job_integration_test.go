package postgres

import (
	"context"
	"testing"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

// The broker as factory.ConnectJob builds it: Postgres adapter wrapped in
// ObservedBroker, history in job_events through the Postgres JobLog.
func TestJobBrokerLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := utils.WithCorrelationID(utils.WithLocalService(context.Background(), "ingest"), "corr-1")
	broker := job.NewObservedBroker(NewPgJobBrokerAdapter(pool), NewPgJobLogAdapter(pool))
	jobType := "test.lifecycle"
	// A queue of its own keeps other tests' jobs out of the claim.
	queue := "test-" + time.Now().Format("150405.000000")

	// Enqueue twice with one key: one job.
	first, err := broker.Enqueue(ctx, jobType, []byte(`{"n":1}`), job.WithIdempotencyKey("k1"), job.WithQueue(queue))
	if err != nil {
		t.Fatal(err)
	}
	again, err := broker.Enqueue(ctx, jobType, []byte(`{"n":1}`), job.WithIdempotencyKey("k1"), job.WithQueue(queue))
	if err != nil || !again.Duplicate || again.JobID != first.JobID {
		t.Fatalf("dedupe: %+v, %v", again, err)
	}

	// Another queue never sees it.
	if _, err := broker.ClaimJob(ctx, []string{queue + "-other"}, "worker-x"); !errs.HasCode(err, errs.CodeJobsUnavailable) {
		t.Fatalf("other queue: err = %v", err)
	}
	// Without a queue, Enqueue is refused.
	if _, err := broker.Enqueue(ctx, jobType, []byte(`{}`)); !errs.HasCode(err, errs.CodeJobInvalidOption) {
		t.Fatalf("no queue: err = %v", err)
	}
	// The same key on another queue is a separate copy (fan-out).
	copy, err := broker.Enqueue(ctx, jobType, []byte(`{"n":1}`), job.WithIdempotencyKey("k1"), job.WithQueue(queue+"-fanout"))
	if err != nil || copy.Duplicate || copy.JobID == first.JobID {
		t.Fatalf("fan-out copy: %+v, %v", copy, err)
	}

	claimed, err := broker.ClaimJob(ctx, []string{queue + "-unused", queue}, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ID != first.JobID || claimed.Queue != queue {
		t.Fatalf("claimed %+v, want job %s on %s", claimed, first.JobID, queue)
	}
	if claimed.Status != job.RunningJobStatus || claimed.Attempt != 1 ||
		claimed.Metadata.CorrelationID != "corr-1" || claimed.Metadata.OriginService != "ingest" {
		t.Fatalf("claimed = %+v", claimed)
	}

	claim := job.Claim{JobID: claimed.ID, WorkerID: "worker-1", Attempt: claimed.Attempt}
	if err := broker.MarkCompleted(ctx, claim, []byte(`{"ok":true}`), 1500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	// The claim is spent: a second write is refused.
	if err := broker.MarkDead(ctx, claim, "late", "late"); !errs.HasCode(err, errs.CodeJobClaimLost) {
		t.Fatalf("after completion: err = %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, first.JobID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" {
		t.Fatalf("status = %q", status)
	}

	rows, err := pool.Query(ctx, `SELECT type, actor FROM job_events WHERE job_id = $1 ORDER BY id`, first.JobID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var typ, actor string
		if err := rows.Scan(&typ, &actor); err != nil {
			t.Fatal(err)
		}
		got = append(got, typ+"/"+actor)
	}
	want := []string{"enqueued/ingest", "claimed/worker-1", "completed/worker-1"}
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events = %v, want %v", got, want)
		}
	}
}

func TestJobTreesAreScopedToTheBudget(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	broker := NewPgJobBrokerAdapter(pool)
	store := NewPgJobStoreAdapter(pool)
	mine, other := newSeed(t, pool).budgetID, newSeed(t, pool).budgetID
	scope := job.JobScope{BudgetID: mine, Accounts: []string{"me@gmail.com"}}
	queue := "test-" + time.Now().Format("150405.000000")

	enqueue := func(jobType string, payload string, opts ...job.EnqueueOption) uuid.UUID {
		t.Helper()
		res, err := broker.Enqueue(ctx, jobType, []byte(payload), append(opts, job.WithQueue(queue))...)
		if err != nil {
			t.Fatal(err)
		}
		return res.JobID
	}
	under := func(root uuid.UUID, budget *uuid.UUID) []job.EnqueueOption {
		return []job.EnqueueOption{job.WithRootJobID(&root), job.WithParentID(&root), job.WithBudgetID(budget)}
	}

	// A mailbox feeding this budget, stored with different casing.
	mailbox := enqueue(model.JobTypeMailboxChanged, `{"provider":"gmail","account":"ME@gmail.com"}`)
	ownMessage := enqueue(model.JobTypeMessageReceived, `{}`, under(mailbox, &mine)...)
	// The same mailbox also feeds another budget: that job must stay hidden.
	theirMessage := enqueue(model.JobTypeMessageReceived, `{}`, under(mailbox, &other)...)
	// Another budget's own job and an unrelated mailbox.
	theirs := enqueue(model.JobTypeMessageReceived, `{}`, job.WithBudgetID(&other))
	strangerMailbox := enqueue(model.JobTypeMailboxChanged, `{"provider":"gmail","account":"stranger@gmail.com"}`)
	// A detached mailbox: no longer in Accounts, but it led to this budget's job.
	detached := enqueue(model.JobTypeMailboxChanged, `{"provider":"gmail","account":"old@gmail.com"}`)
	enqueue(model.JobTypeMessageReceived, `{}`, under(detached, &mine)...)

	trees, total, err := store.ListTrees(ctx, job.JobTreeFilter{Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	roots := map[uuid.UUID]job.JobTree{}
	for _, tree := range trees {
		roots[tree.Root.ID] = tree
	}
	if total != 2 || len(roots) != 2 {
		t.Fatalf("total = %d, roots = %v", total, roots)
	}
	if _, ok := roots[theirs]; ok {
		t.Fatal("another budget's job is visible")
	}
	if _, ok := roots[strangerMailbox]; ok {
		t.Fatal("an unrelated mailbox is visible")
	}
	if _, ok := roots[detached]; !ok {
		t.Fatal("a detached mailbox with this budget's job is hidden")
	}
	members := roots[mailbox].Jobs
	if len(members) != 2 || members[0].ID != mailbox || members[1].ID != ownMessage {
		t.Fatalf("mailbox tree = %+v", members)
	}
	for _, j := range members {
		if j.ID == theirMessage {
			t.Fatal("another budget's job leaked through the shared root")
		}
	}

	// Fail the message for good: the filter returns the tree around it, and the
	// failure is grouped by its code.
	claimed, err := broker.ClaimJob(ctx, []string{queue}, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	for claimed.ID != ownMessage {
		if claimed, err = broker.ClaimJob(ctx, []string{queue}, "worker-1"); err != nil {
			t.Fatal(err)
		}
	}
	claim := job.Claim{JobID: claimed.ID, WorkerID: "worker-1", Attempt: claimed.Attempt}
	if err := broker.MarkDead(ctx, claim, "MESSAGE_CHECK_FAILED: check transaction: boom", "exhausted"); err != nil {
		t.Fatal(err)
	}

	for name, filter := range map[string]job.JobTreeFilter{
		"status": {Scope: scope, Status: job.DeadJobStatus},
		"code":   {Scope: scope, ErrorCode: "MESSAGE_CHECK_FAILED"},
		"job id": {Scope: scope, JobID: &ownMessage},
		"type":   {Scope: scope, Type: model.JobTypeMailboxChanged, Status: ""},
	} {
		trees, _, err := store.ListTrees(ctx, filter)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		found := false
		for _, tree := range trees {
			found = found || tree.Root.ID == mailbox
		}
		if !found {
			t.Fatalf("%s: mailbox tree missing from %+v", name, trees)
		}
	}
	if trees, _, err := store.ListTrees(ctx, job.JobTreeFilter{Scope: scope, JobID: &theirs}); err != nil || len(trees) != 0 {
		t.Fatalf("another budget's job ID: %+v, %v", trees, err)
	}

	counts, failures, err := store.Overview(ctx, scope, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if counts.Failed != 1 || len(failures) != 1 || failures[0].Code != "MESSAGE_CHECK_FAILED" ||
		failures[0].Count != 1 || failures[0].Message != "MESSAGE_CHECK_FAILED: check transaction: boom" {
		t.Fatalf("counts = %+v, failures = %+v", counts, failures)
	}
	otherCounts, otherFailures, err := store.Overview(ctx, job.JobScope{BudgetID: other}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if otherCounts.Failed != 0 || len(otherFailures) != 0 {
		t.Fatalf("other budget sees %+v, %+v", otherCounts, otherFailures)
	}
}

func TestJobEventsReadBack(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	broker := job.NewObservedBroker(NewPgJobBrokerAdapter(pool), NewPgJobLogAdapter(pool))
	store := NewPgJobStoreAdapter(pool)
	queue := "test-" + time.Now().Format("150405.000000")

	res, err := broker.Enqueue(ctx, "test.events", []byte(`{}`), job.WithQueue(queue))
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := broker.ClaimJob(ctx, []string{queue}, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	claim := job.Claim{JobID: claimed.ID, WorkerID: "worker-1", Attempt: claimed.Attempt}
	if err := broker.MarkCompleted(ctx, claim, []byte(`{}`), 1500*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	events, err := store.ListEvents(ctx, res.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Type != job.EventEnqueued || events[1].Type != job.EventClaimed ||
		events[2].Type != job.EventCompleted {
		t.Fatalf("events = %+v", events)
	}
	if events[2].DurationMs == nil || *events[2].DurationMs != 1500 {
		t.Fatalf("duration = %v", events[2].DurationMs)
	}
}

func TestJobStore_ListTreesHidesEmptySyncs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	broker := NewPgJobBrokerAdapter(pool)
	store := NewPgJobStoreAdapter(pool)
	mine, other := newSeed(t, pool).budgetID, newSeed(t, pool).budgetID
	// Its own address: the tests share a database, and another test's mailbox
	// jobs would be in this scope too.
	scope := job.JobScope{BudgetID: mine, Accounts: []string{"empty-syncs@gmail.com"}}
	queue := "test-" + time.Now().Format("150405.000000")

	sync := func(status job.JobStatus) uuid.UUID {
		t.Helper()
		res, err := broker.Enqueue(ctx, model.JobTypeMailboxChanged,
			[]byte(`{"provider":"gmail","account":"empty-syncs@gmail.com"}`), job.WithQueue(queue))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE jobs SET status = $1 WHERE id = $2`, string(status), res.JobID); err != nil {
			t.Fatal(err)
		}
		return res.JobID
	}
	message := func(root uuid.UUID, budget *uuid.UUID) {
		t.Helper()
		if _, err := broker.Enqueue(ctx, model.JobTypeMessageReceived, []byte(`{}`), job.WithQueue(queue),
			job.WithRootJobID(&root), job.WithParentID(&root), job.WithBudgetID(budget)); err != nil {
			t.Fatal(err)
		}
	}

	empty := sync(job.SucceededJobStatus)
	onlyTheirs := sync(job.SucceededJobStatus)
	message(onlyTheirs, &other)
	withEmail := sync(job.SucceededJobStatus)
	message(withEmail, &mine)
	running := sync(job.RunningJobStatus)
	broken := sync(job.DeadJobStatus)

	trees, total, err := store.ListTrees(ctx, job.JobTreeFilter{Scope: scope, HideEmptySyncs: true})
	if err != nil {
		t.Fatal(err)
	}
	listed := map[uuid.UUID]bool{}
	for _, tree := range trees {
		listed[tree.Root.ID] = true
	}
	for name, id := range map[string]uuid.UUID{"withEmail": withEmail, "running": running, "broken": broken} {
		if !listed[id] {
			t.Errorf("%s sync is missing", name)
		}
	}
	for name, id := range map[string]uuid.UUID{"empty": empty, "onlyTheirs": onlyTheirs} {
		if listed[id] {
			t.Errorf("%s sync is still listed", name)
		}
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}

func TestJobStore_ListTreesHiddenRootTypes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	broker := NewPgJobBrokerAdapter(pool)
	store := NewPgJobStoreAdapter(pool)
	mine := newSeed(t, pool).budgetID
	scope := job.JobScope{BudgetID: mine, Accounts: []string{"me@gmail.com"}}
	queue := "test-" + time.Now().Format("150405.000000")

	enqueue := func(jobType string, payload string, opts ...job.EnqueueOption) uuid.UUID {
		t.Helper()
		res, err := broker.Enqueue(ctx, jobType, []byte(payload), append(opts, job.WithQueue(queue))...)
		if err != nil {
			t.Fatal(err)
		}
		return res.JobID
	}
	under := func(root uuid.UUID, budget *uuid.UUID) []job.EnqueueOption {
		return []job.EnqueueOption{job.WithRootJobID(&root), job.WithParentID(&root), job.WithBudgetID(budget)}
	}

	root := enqueue(model.JobTypeTransactionLearn, `{}`, job.WithBudgetID(&mine))
	parent := enqueue(model.JobTypeMessageReceived, `{}`, job.WithBudgetID(&mine))
	child := enqueue(model.JobTypeTransactionLearn, `{}`, under(parent, &mine)...)

	trees, _, err := store.ListTrees(ctx, job.JobTreeFilter{
		Scope:           scope,
		HiddenRootTypes: []string{model.JobTypeTransactionLearn},
	})
	if err != nil {
		t.Fatal(err)
	}
	var pipeline *job.JobTree
	for i := range trees {
		if trees[i].Root.ID == root {
			t.Fatal("a standalone learn job is still listed")
		}
		if trees[i].Root.ID == parent {
			pipeline = &trees[i]
		}
	}
	if pipeline == nil || len(pipeline.Jobs) != 2 || pipeline.Jobs[1].ID != child {
		t.Fatalf("the learn job inside a pipeline should stay: %+v", pipeline)
	}
}
