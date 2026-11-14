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

	// JobTypeMailboxStartWatch / JobTypeMailboxStopWatch: start (or renew)
	// and stop a connection's provider watch.
	JobTypeMailboxStartWatch = "mailbox.watch.start"
	JobTypeMailboxStopWatch  = "mailbox.watch.stop"

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

	// JobTypeStatementFetch: download the PDF attachments of an email the
	// check step called a statement. Queued by cipher on QueueIngest;
	// payload StatementFetchPayload.
	JobTypeStatementFetch = "statement.fetch"
	// JobTypeStatementImport: create a statement import from a fetched PDF
	// and queue its statement.extract with the budget's saved passwords.
	// Queued by ingest on QueuePennywise; payload StatementImportPayload.
	JobTypeStatementImport = "statement.import"

	// JobTypePipelineReport: record how one message ended. Queued on
	// QueuePennywise by whichever step ended it; payload PipelineReportPayload.
	JobTypePipelineReport = "pipeline.report"

	// JobTypeStatementExtract: read the rows out of an uploaded statement PDF.
	// Queued on [QueueCipher] by the API's statement upload; payload
	// StatementInput. Queues JobTypeStatementSave with what it read.
	JobTypeStatementExtract = "statement.extract"

	// JobTypeStatementSave: save extracted rows onto their statement import.
	// Queued on [QueuePennywise] by statement.extract; payload
	// StatementSavePayload.
	JobTypeStatementSave = "statement.save"
)
