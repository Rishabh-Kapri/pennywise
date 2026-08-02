/**
 * Minimal test harness. Deliberately dependency-free: these tests run on Node's
 * type stripping via `test/loader.mjs`, so the app gains no test-runner dep and
 * the suite exercises the real source files rather than copies.
 */

let failures = 0;

export async function test(name: string, fn: () => Promise<void>): Promise<void> {
  try {
    await fn();
    console.log(`  PASS  ${name}`);
  } catch (err) {
    failures++;
    console.log(`  FAIL  ${name}\n        ${(err as Error).message}`);
  }
}

export function finish(): never {
  console.log(failures === 0 ? '\nALL PASS' : `\n${failures} FAILURE(S)`);
  process.exit(failures === 0 ? 0 : 1);
}

export type StubResponse = { status: number; body: unknown };
export type Responder = (url: string, init: RequestInit) => StubResponse | null;

export type RecordedCall = {
  url: string;
  method: string;
  headers: Record<string, string>;
  body?: string;
};

export type FetchController = {
  calls: RecordedCall[];
  respond: (...responders: Responder[]) => void;
  reset: () => void;
};

/** Replaces global fetch with a recorder driven by ordered responders. */
export function installFetch(): FetchController {
  const state: { calls: RecordedCall[]; responders: Responder[] } = { calls: [], responders: [] };

  globalThis.fetch = (async (url: string, init: RequestInit = {}) => {
    state.calls.push({
      url,
      method: init.method ?? 'GET',
      headers: (init.headers ?? {}) as Record<string, string>,
      body: init.body as string | undefined
    });

    for (const responder of state.responders) {
      const res = responder(url, init);
      if (res) {
        return {
          ok: res.status >= 200 && res.status < 300,
          status: res.status,
          statusText: String(res.status),
          text: async () => JSON.stringify(res.body),
          json: async () => res.body
        };
      }
    }
    throw new Error(`unstubbed fetch: ${url}`);
  }) as unknown as typeof fetch;

  return {
    calls: state.calls,
    respond: (...responders: Responder[]) => {
      state.responders = responders;
    },
    reset: () => {
      state.calls.length = 0;
      state.responders = [];
    }
  };
}

/** Responder helper: matches a URL substring and returns a 200 with `body`. */
export const okFor =
  (match: string, body: unknown = {}): Responder =>
  (url) =>
    url.includes(match) ? { status: 200, body } : null;

/** Responder helper: matches a URL substring and returns `status` with `body`. */
export const failFor =
  (match: string, status: number, body: unknown = {}): Responder =>
  (url) =>
    url.includes(match) ? { status, body } : null;
