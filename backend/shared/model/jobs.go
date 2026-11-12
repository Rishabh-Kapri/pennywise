package model

// Queue names and job types. There is no routing: every Enqueue names its
// queue (job.WithQueue), and a worker claims from the queues it is given.

// Queues.
const (
	// QueueCipher: cipher's worker. Classifies messages: is it a
	// transaction, extract the details, predict payee and category.
	QueueCipher = "cipher"
	// QueueIngest: ingest's worker. Syncs connected mailboxes and manages
	// their provider watches.
	QueueIngest = "ingest"
	// QueuePennywise: go-pennywise-api's worker. Writes what cipher decided:
	// the transaction and the pipeline run history.
	QueuePennywise = "pennywise"
)

// Job types. The payload of each is noted next to it.
const (
	// JobTypeMessageReceived: one accepted message, from any source (email,
	// SMS, API), for one budget (the job's BudgetID). Queued by ingest;
	// payload ReceivedMessage.
	JobTypeMessageReceived = "message.received"

	// JobTypeMailboxChanged: a mail provider reported new activity in a
	// connected mailbox (a Gmail push). The handler fetches from the
	// connection's stored cursor. Queued by ingest's push receiver; payload
	// MailboxChanged.
	JobTypeMailboxChanged = "mailbox.changed"

	// JobTypeMailboxBackfill: import emails the user chose from a connected
	// mailbox. Each becomes a message.received child. Queued on QueueIngest by
	// POST /api/ingest/connections/:id/backfill; payload MailboxBackfill.
	JobTypeMailboxBackfill = "mailbox.backfill"

	// JobTypeMailboxStartWatch: start (or renew) a connection's provider
	// watch. Stopping one is done in the request, not as a job.
	JobTypeMailboxStartWatch = "mailbox.watch.start"

	// JobTypeMessageExtract, JobTypeMessageVerify, JobTypeMessageLookup,
	// JobTypeMessagePredict: cipher's classification steps (extract fields,
	// check them against the message, look up memory and rules, predict).
	// Queued by cipher on QueueCipher; payloads in messagePipeline.go.
	JobTypeMessageExtract = "message.extract"
	JobTypeMessageVerify  = "message.verify"
	JobTypeMessageLookup  = "message.lookup"
	JobTypeMessagePredict = "message.predict"

	// JobTypeTransactionCreate: create the transaction and its cipher
	// prediction, then tell the clients. Queued by cipher on QueuePennywise;
	// payload TransactionCreatePayload.
	JobTypeTransactionCreate = "transaction.create"

	// JobTypeTransactionLearn: teach the lookup step who a transaction was, after
	// the pipeline creates one or the user corrects or approves one. On
	// QueuePennywise; payload TransactionLearnPayload.
	JobTypeTransactionLearn = "transaction.learn"

	// JobTypePipelineReport: record how one message ended. Queued on
	// QueuePennywise by whichever step ended it; payload PipelineReportPayload.
	JobTypePipelineReport = "pipeline.report"
)
