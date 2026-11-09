import { useAppSelector } from '@/app/hooks';
import { apiClient, errorMessage, toast } from '@/utils';
import { useGoogleOAuth } from '@react-oauth/google';
import { useEffect, useState } from 'react';

const GMAIL_READ_SCOPE = 'https://www.googleapis.com/auth/gmail.readonly';

interface CodeClient {
  requestCode: () => void;
}

interface CodeResponse {
  code?: string;
  error?: string;
}

interface GoogleOAuth2 {
  initCodeClient: (config: object) => CodeClient;
}

/**
 * Called directly because useGoogleLogin fixes its options once, and the
 * account to ask for changes per click. @react-oauth/google loads the script.
 */
function googleOAuth2() {
  return (window as unknown as { google?: { accounts?: { oauth2?: GoogleOAuth2 } } }).google?.accounts?.oauth2;
}

export interface MailConnection {
  id: string;
  provider: string;
  account: string;
  status: 'active' | 'needs_reconnect' | 'error';
  lastSyncAt?: string;
  lastError?: string;
  pausedAt?: string;
}

/** The selected budget's mailboxes, and Google's consent flow for adding one. */
export function useMailConnections() {
  const email = useAppSelector((state) => state.auth.user?.email);
  const [connections, setConnections] = useState<MailConnection[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [connecting, setConnecting] = useState(false);
  const { clientId, scriptLoadedSuccessfully } = useGoogleOAuth();

  useEffect(() => {
    apiClient
      .get<MailConnection[]>('mail-connections')
      .then((list) => setConnections(list ?? []))
      .catch((err: unknown) => setLoadError(errorMessage(err, "Couldn't load your mailboxes.")));
  }, []);

  const saveConnection = (connection: MailConnection) => {
    setConnections((current) => {
      const list = current ?? [];
      return list.some(({ id }) => id === connection.id)
        ? list.map((existing) => (existing.id === connection.id ? connection : existing))
        : [...list, connection];
    });
  };

  const finishConnecting = async ({ code, error }: CodeResponse) => {
    if (error || !code) {
      toast.error("Couldn't connect Gmail", { description: 'Google sign-in failed. Please try again.' });
      return;
    }
    setConnecting(true);
    try {
      const connected = await apiClient.post<MailConnection, { code: string }>('mail-connections', { code });
      saveConnection(connected);
      setLoadError(null);
      toast.success('Gmail connected', { description: connected.account });
    } catch (err) {
      toast.error("Couldn't connect Gmail", { description: errorMessage(err, 'Please try again.') });
    } finally {
      setConnecting(false);
    }
  };

  /**
   * Asks Google for read access to `account`, or to one the user picks. The
   * first mailbox is usually the login account, so it's suggested. Google only
   * returns a refresh token from its consent screen, which it skips for a user
   * who already granted access, so consent is always asked for.
   */
  const connect = (account?: string) => {
    const oauth2 = googleOAuth2();
    if (!oauth2 || !scriptLoadedSuccessfully) {
      toast.error("Couldn't connect Gmail", { description: "Google sign-in hasn't loaded. Please try again." });
      return;
    }
    const hint = account ?? (connections?.length ? undefined : email);
    oauth2
      .initCodeClient({
        client_id: clientId,
        scope: GMAIL_READ_SCOPE,
        // Keeps this grant to read-only, even for users who logged in with more.
        include_granted_scopes: false,
        hint,
        prompt: hint ? 'consent' : 'select_account consent',
        callback: finishConnecting,
      })
      .requestCode();
  };

  const setPaused = async (connection: MailConnection, paused: boolean) => {
    try {
      saveConnection(
        await apiClient.patch<MailConnection, { paused: boolean }>(`mail-connections/${connection.id}`, { paused }),
      );
    } catch (err) {
      const title = paused ? "Couldn't pause this mailbox" : "Couldn't resume this mailbox";
      toast.error(title, { description: errorMessage(err, 'Please try again.') });
    }
  };

  const disconnect = async (connection: MailConnection) => {
    try {
      await apiClient.delete(`mail-connections/${connection.id}`);
      setConnections((current) => (current ?? []).filter(({ id }) => id !== connection.id));
      toast.success('Gmail disconnected', { description: connection.account });
    } catch (err) {
      toast.error("Couldn't disconnect this mailbox", { description: errorMessage(err, 'Please try again.') });
    }
  };

  return { connections, loadError, connecting, connect, setPaused, disconnect };
}
