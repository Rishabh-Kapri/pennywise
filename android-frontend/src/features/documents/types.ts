/** Filter for the budget-wide document library. */
export type DocumentKind = '' | 'image' | 'pdf';

/**
 * A document plus the transaction context needed to render a library tile.
 * Mirrors `model.DocumentListItem` on the API — the embedded document fields
 * arrive flattened into the same JSON object by Go's struct embedding.
 */
export interface DocumentListItem {
  id: string;
  budgetId: string;
  transactionId: string;
  fileName: string;
  mimeType: string;
  sizeBytes: number;
  createdAt: string;
  updatedAt: string;
  transactionDate: string;
  transactionAmount: number;
  payeeName?: string;
  accountName?: string;
  categoryName?: string;
}

export interface DocumentLibraryResponse {
  data: DocumentListItem[];
  total: number;
  hasMore: boolean;
}

export interface DocumentFilters {
  search: string;
  kind: DocumentKind;
  startDate: string;
  endDate: string;
}

export const EMPTY_DOCUMENT_FILTERS: DocumentFilters = {
  search: '',
  kind: '',
  startDate: '',
  endDate: ''
};

/** Builds the query string for GET /api/documents. */
export function buildDocumentQuery(filters: DocumentFilters, offset: number, limit: number): string {
  const parts: string[] = [];
  const search = filters.search.trim();
  if (search) parts.push(`search=${encodeURIComponent(search)}`);
  if (filters.kind) parts.push(`type=${filters.kind}`);
  if (filters.startDate) parts.push(`startDate=${filters.startDate}`);
  if (filters.endDate) parts.push(`endDate=${filters.endDate}`);
  parts.push(`limit=${limit}`);
  parts.push(`offset=${offset}`);
  return parts.join('&');
}
