import { useCallback, useEffect, useRef, useState } from 'react';
import { apiClient } from '@/utils';
import type { DocumentFile } from '../types/document.types';

/**
 * Object URLs for the image documents on screen, keyed by document id. They
 * go through fetchBlobUrl because a plain <img src> can't carry the auth
 * header. Every URL is revoked when it's forgotten, cleared or unmounted.
 */
export function useImagePreviews() {
  const [previews, setPreviews] = useState<Record<string, string>>({});
  const urlsRef = useRef<Record<string, string>>({});

  const clear = useCallback(() => {
    for (const url of Object.values(urlsRef.current)) URL.revokeObjectURL(url);
    urlsRef.current = {};
    setPreviews({});
  }, []);

  /** `isCurrent` turns false when the list moved on while previews were loading. */
  const load = useCallback(async (docs: DocumentFile[], isCurrent: () => boolean = () => true) => {
    for (const doc of docs) {
      if (!doc.mimeType.startsWith('image/') || urlsRef.current[doc.id]) continue;
      try {
        const url = await apiClient.fetchBlobUrl(`documents/${doc.id}/content`);
        if (!isCurrent()) {
          URL.revokeObjectURL(url);
          return;
        }
        urlsRef.current[doc.id] = url;
        setPreviews({ ...urlsRef.current });
      } catch {
        // a missing preview is non-fatal; the tile falls back to a file icon
      }
    }
  }, []);

  const forget = useCallback((documentId: string) => {
    const url = urlsRef.current[documentId];
    if (!url) return;
    URL.revokeObjectURL(url);
    delete urlsRef.current[documentId];
    setPreviews({ ...urlsRef.current });
  }, []);

  useEffect(() => clear, [clear]);

  return { previews, load, forget, clear };
}

/** Opens a document in a new tab. */
export function openDocument(doc: DocumentFile): Promise<void> {
  return apiClient.openFile(`documents/${doc.id}/content`);
}
