/**
 * Server errors arrive as a chain of codes and messages, outermost first:
 * "MESSAGE_EXTRACT_FAILED: extract entities: MESSAGE_EXTRACT_FAILED: qwen3.5:9b
 * reached its token limit". explainError turns one into words for a person.
 */

export interface ExplainedError {
  /** What went wrong, in a short sentence. */
  title: string;
  /** The reason underneath, when there is one worth showing. */
  detail?: string;
  /** The outermost server code, e.g. MESSAGE_EXTRACT_FAILED. */
  code?: string;
}

interface KnownError {
  title: string;
  /** Replaces the server's own wording when that says nothing useful. */
  detail?: string;
}

/** Codes a person can run into. Others fall back to the server's message. */
const KNOWN: Record<string, KnownError> = {
  MESSAGE_CHECK_FAILED: { title: "Couldn't check whether this is a transaction" },
  MESSAGE_EXTRACT_FAILED: { title: "Couldn't read the details from the message" },
  MESSAGE_VERIFY_FAILED: { title: "Couldn't verify the details" },
  MESSAGE_LOOKUP_FAILED: { title: "Couldn't look up the merchant" },
  MESSAGE_PREDICT_FAILED: { title: "Couldn't pick a payee and category" },
  PIPELINE_REPORT_FAILED: { title: "Couldn't record how the message ended" },
  TRANSACTION_CREATE_FAILED: { title: "Couldn't create the transaction" },
  TRANSACTION_LEARN_FAILED: { title: "Couldn't learn from the transaction" },
  STATEMENT_EXTRACT_FAILED: { title: "Couldn't read the statement" },
  STATEMENT_PASSWORD_REQUIRED: {
    title: 'This PDF is locked',
    detail: "None of the passwords opened it. Upload it again with its password, or save the password on the account.",
  },
  AI_PROVIDER_NOT_CONFIGURED: { title: 'No AI model is set up for this' },
  AGENT_LLM_NOT_CONFIGURED: { title: 'No AI model is set up for this' },
  DECISIONS_NOT_CONFIGURED: { title: 'No decision model is set up' },
  DECISIONS_REQUEST_FAILED: { title: "The decision model didn't answer" },
  DECISIONS_ANSWER_MISSING: { title: 'The decision model gave no answer' },
  MAIL_AUTH_FAILED: { title: 'The mailbox sign-in no longer works' },
  MAIL_NO_REFRESH_TOKEN: { title: "The mailbox didn't give long-term access" },
  MAIL_CURSOR_EXPIRED: { title: 'The mailbox sync fell too far behind' },
  MAIL_REQUEST_FAILED: { title: "Couldn't reach the mailbox" },
  MAIL_NOT_CONFIGURED: { title: "Email sync isn't set up on this server" },
  JOB_HANDLER_ERROR: { title: 'The job failed' },
  JOB_CLAIM_LOST: { title: 'The job was interrupted' },
  JOB_INVALID_PAYLOAD: { title: "The job's data couldn't be read" },
  JOB_PAYLOAD_PARSE_FAILED: { title: "The job's data couldn't be read" },
  JOB_ENQUEUE_FAILED: { title: "Couldn't queue the job" },
  JOBS_UNAVAILABLE: { title: 'The job queue is off' },
  ADAPTER_NOT_CONFIGURED: { title: "This isn't set up on the server" },
  HTTP_CLIENT_ERROR: { title: "Couldn't reach the service" },
};

const CODE = /^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$/;
/** What a failed command leaves behind; it says nothing to a person. */
const NOISE = /^(exit status \d+|signal: \w+)$/i;

function sentence(text: string): string {
  const trimmed = text.trim().replace(/[.:\s]+$/, '');
  return trimmed ? trimmed[0].toUpperCase() + trimmed.slice(1) : '';
}

/** "MESSAGE_EXTRACT_FAILED" -> "Message extract failed", for codes with no entry. */
function fromCode(code: string): string {
  return sentence(code.toLowerCase().replace(/_/g, ' '));
}

export function explainError(raw: string | null | undefined): ExplainedError {
  const text = (raw ?? '').trim();
  if (!text) return { title: 'Something went wrong' };

  const parts = text
    .split(': ')
    .map((part) => part.trim())
    .filter(Boolean);
  const code = parts.find((part) => CODE.test(part));
  // No code: it's already a sentence someone wrote.
  if (!code) return { title: text };

  const words = parts.filter((part, index) => !CODE.test(part) && !NOISE.test(part) && parts.indexOf(part) === index);
  const known = KNOWN[code];
  // The last message in the chain is the cause; the first is the server's
  // own description of the failure.
  const cause = words.length ? sentence(words[words.length - 1]) : '';

  if (known) {
    const detail = known.detail ?? cause;
    return { title: known.title, detail: detail || undefined, code };
  }
  const title = words.length ? sentence(words[0]) : fromCode(code);
  return { title, detail: cause && cause !== title ? cause : undefined, code };
}

/** The same, on one line: for toasts and inline error text. */
export function errorText(raw: string | null | undefined): string {
  const { title, detail } = explainError(raw);
  return detail ? `${title}: ${lowerFirst(detail)}` : title;
}

/** Keeps a leading name or acronym ("Gmail", "PDF") as written. */
function lowerFirst(text: string): string {
  const first = text.split(' ')[0];
  return /^[A-Z][a-z]+$/.test(first) && !/^(Gmail|Google|Ollama)$/.test(first)
    ? text[0].toLowerCase() + text.slice(1)
    : text;
}
