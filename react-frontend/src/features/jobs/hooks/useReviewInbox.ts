import { useCallback, useEffect, useRef, useState } from 'react';
import { useAppSelector } from '@/app/hooks';
import { selectSelectedBudget } from '@/features/budget/store/budgetSlice';
import { apiClient } from '@/utils';
import type { InboundMessage, InboundMessageList } from '../types/job.types';

/** How long a burst of job updates waits before one reload. */
const RELOAD_MS = 2000;
const PAGE = 20;

export interface ReviewInbox {
  items: InboundMessage[];
  /** Every message waiting, not only the page in items. */
  count: number;
  loaded: boolean;
  reload: () => void;
}

/**
 * The messages parked for review (`needs_review`), for the navbar. Loads on
 * budget change and again shortly after job updates arrive over the websocket,
 * since a run that parks or a review that's settled changes the list.
 */
export function useReviewInbox(): ReviewInbox {
  const budgetId = useAppSelector(selectSelectedBudget)?.id;
  const liveVersion = useAppSelector((state) => state.jobs.liveVersion);
  const [items, setItems] = useState<InboundMessage[]>([]);
  const [count, setCount] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const request = useRef(0);

  const reload = useCallback(() => {
    if (!budgetId) return;
    const id = ++request.current;
    apiClient
      .get<InboundMessageList>(`messages?status=needs_review&limit=${PAGE}`)
      .then((list) => {
        if (id !== request.current) return;
        setItems(list.items ?? []);
        setCount(list.counts?.needs_review ?? list.items?.length ?? 0);
        setLoaded(true);
      })
      // A failed load keeps what was shown; the next update tries again.
      .catch(() => undefined);
  }, [budgetId]);

  useEffect(() => {
    reload();
  }, [reload]);

  useEffect(() => {
    if (liveVersion === 0) return;
    const timer = window.setTimeout(reload, RELOAD_MS);
    return () => window.clearTimeout(timer);
  }, [liveVersion, reload]);

  return { items, count, loaded, reload };
}
