import * as FileSystem from 'expo-file-system/legacy';
import { apiClient } from '../../utils/api';

/**
 * Viewing and saving a document are deliberately different operations.
 *
 * **Viewing** pulls the body into the app's own cache directory and renders it
 * from there. The cache is private to the app, is evicted by Android under
 * storage pressure, and never appears in the gallery or Downloads — so opening
 * a receipt to look at it does not leave a copy on the device.
 *
 * **Saving** is the explicit user action: it copies that same body out to a
 * folder the user picks through the Storage Access Framework, which is the only
 * way to write outside the sandbox without a legacy storage permission.
 */

/** Extension carried by the stored file name, defaulted per mime type. */
function extensionFor(fileName: string, mimeType: string): string {
  const dot = fileName.lastIndexOf('.');
  if (dot > 0 && dot < fileName.length - 1) {
    return fileName.slice(dot);
  }
  if (mimeType === 'application/pdf') return '.pdf';
  if (mimeType === 'image/png') return '.png';
  if (mimeType === 'image/webp') return '.webp';
  if (mimeType === 'image/heic') return '.heic';
  return '.jpg';
}

function baseName(fileName: string): string {
  const dot = fileName.lastIndexOf('.');
  return dot > 0 ? fileName.slice(0, dot) : fileName;
}

export interface DocumentFileRef {
  id: string;
  fileName: string;
  mimeType: string;
}

/** Cache path a document's body is read from; shared across every screen. */
export function documentCacheUri(doc: DocumentFileRef): string {
  return `${FileSystem.cacheDirectory}receipt-${doc.id}${extensionFor(doc.fileName, doc.mimeType)}`;
}

/**
 * Makes the document body available locally for viewing, downloading it into
 * the app cache only if it is not already there. Returns the local file URI.
 */
export async function ensureDocumentCached(doc: DocumentFileRef): Promise<string> {
  const target = documentCacheUri(doc);

  const info = await FileSystem.getInfoAsync(target);
  if (info.exists && info.size > 0) {
    return target;
  }

  const { url, headers } = apiClient.getAuthorizedRequest(`documents/${doc.id}/content`);
  const result = await FileSystem.downloadAsync(url, target, { headers });
  if (result.status !== 200) {
    throw new Error(`Could not load this document (HTTP ${result.status})`);
  }
  return target;
}

export type SaveResult =
  | { status: 'saved'; fileName: string }
  | { status: 'cancelled' };

/**
 * Copies the document into a user-chosen folder via the Storage Access
 * Framework. Android returns no path for a SAF document, only an opaque URI,
 * so the caller can confirm the file name but not where it landed.
 */
export async function saveDocumentToDevice(doc: DocumentFileRef): Promise<SaveResult> {
  const cached = await ensureDocumentCached(doc);

  const permission = await FileSystem.StorageAccessFramework.requestDirectoryPermissionsAsync();
  if (!permission.granted) {
    return { status: 'cancelled' };
  }

  const body = await FileSystem.readAsStringAsync(cached, {
    encoding: FileSystem.EncodingType.Base64
  });

  // SAF appends the extension implied by the mime type, so hand it the stem
  // only or the saved file ends up as "receipt.pdf.pdf".
  const target = await FileSystem.StorageAccessFramework.createFileAsync(
    permission.directoryUri,
    baseName(doc.fileName),
    doc.mimeType
  );
  await FileSystem.writeAsStringAsync(target, body, {
    encoding: FileSystem.EncodingType.Base64
  });

  return { status: 'saved', fileName: doc.fileName };
}
