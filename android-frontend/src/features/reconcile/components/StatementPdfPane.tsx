import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, TextInput, View, useWindowDimensions } from 'react-native';
import Pdf, { type PdfRef } from 'react-native-pdf';
import { Maximize2, Minimize2, X } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { colors, radii, spacing } from '../../../theme';
import { ensureDocumentCached } from '../../documents/documentFile';

/** A page to turn to, 1-based; `at` makes asking for the same page again count. */
export interface PageRequest {
  page: number;
  at: number;
}

/**
 * The statement's PDF pinned above its parsed rows, to check one against the
 * other. Rows that know their page jump it there. The file is the one that was
 * uploaded, so a locked statement asks for its password here; it's only used
 * to render and is never stored.
 */
export function StatementPdfPane({
  statementId,
  fileName,
  request,
  onClose
}: {
  statementId: string;
  fileName: string;
  request: PageRequest | null;
  onClose: () => void;
}) {
  const { height } = useWindowDimensions();
  const pdf = useRef<PdfRef>(null);
  const [uri, setUri] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [needsPassword, setNeedsPassword] = useState(false);
  const [typed, setTyped] = useState('');
  const [password, setPassword] = useState<string | undefined>(undefined);
  const [page, setPage] = useState(1);
  const [pageCount, setPageCount] = useState(0);
  const [tall, setTall] = useState(false);

  // turn to the requested page once the document is open
  useEffect(() => {
    if (!request || !pageCount) return;
    pdf.current?.setPage(Math.min(Math.max(request.page, 1), pageCount));
  }, [request, pageCount]);

  useEffect(() => {
    let cancelled = false;
    ensureDocumentCached({
      id: statementId,
      fileName,
      mimeType: 'application/pdf',
      contentPath: `statements/${statementId}/file`
    })
      .then((local) => !cancelled && setUri(local))
      .catch((err: unknown) => !cancelled && setError(err instanceof Error ? err.message : 'Could not load the PDF'));
    return () => {
      cancelled = true;
    };
  }, [statementId, fileName]);

  const onPdfError = (err: unknown) => {
    const message = err instanceof Error ? err.message : String(err);
    if (/password/i.test(message)) {
      setNeedsPassword(true);
      setError(password ? 'That password didn\'t open it.' : null);
      return;
    }
    setError('This PDF could not be opened');
  };

  return (
    <View style={[styles.pane, { height: height * (tall ? 0.7 : 0.42) }]}>
      <View style={styles.bar}>
        <AppText variant="caption" muted numberOfLines={1} style={styles.flex}>
          {pageCount ? `Page ${page} of ${pageCount}` : fileName}
        </AppText>
        <Pressable accessibilityLabel={tall ? 'Make the PDF smaller' : 'Make the PDF taller'} hitSlop={8} onPress={() => setTall(!tall)}>
          {tall ? <Minimize2 size={17} color={colors.muted} /> : <Maximize2 size={17} color={colors.muted} />}
        </Pressable>
        <Pressable accessibilityLabel="Close the PDF" hitSlop={8} onPress={onClose}>
          <X size={18} color={colors.muted} />
        </Pressable>
      </View>

      {needsPassword && !password ? (
        <View style={styles.center}>
          <AppText variant="caption" muted style={styles.text}>
            This statement is locked. Enter its password to view it.
          </AppText>
          <TextInput
            value={typed}
            onChangeText={setTyped}
            onSubmitEditing={() => typed && setPassword(typed)}
            secureTextEntry
            autoFocus
            autoCapitalize="none"
            autoCorrect={false}
            returnKeyType="go"
            placeholder="PDF password"
            placeholderTextColor={colors.faint}
            style={styles.input}
          />
          {error ? (
            <AppText variant="caption" tone="danger">
              {error}
            </AppText>
          ) : null}
        </View>
      ) : error && !needsPassword ? (
        <View style={styles.center}>
          <AppText variant="caption" tone="danger" style={styles.text}>
            {error}
          </AppText>
        </View>
      ) : !uri ? (
        <View style={styles.center}>
          <ActivityIndicator color={colors.primary} />
        </View>
      ) : (
        <Pdf
          // a new password needs a fresh document
          key={password ?? 'none'}
          ref={pdf}
          source={{ uri }}
          password={password}
          style={styles.pdf}
          trustAllCerts={false}
          onLoadComplete={(pages) => {
            setPageCount(pages);
            setNeedsPassword(false);
            setError(null);
          }}
          onPageChanged={(current) => setPage(current)}
          onError={(err: unknown) => {
            onPdfError(err);
            if (password) setPassword(undefined);
          }}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  pane: {
    borderRadius: radii.md,
    overflow: 'hidden',
    backgroundColor: colors.surface
  },
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    backgroundColor: colors.surfaceStrong
  },
  flex: { flex: 1 },
  pdf: { flex: 1, backgroundColor: colors.surface },
  center: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: spacing.md, padding: spacing.lg },
  text: { textAlign: 'center' },
  input: {
    alignSelf: 'stretch',
    minHeight: 44,
    paddingHorizontal: spacing.md,
    borderRadius: radii.sm,
    backgroundColor: colors.surfaceStrong,
    color: colors.text,
    fontSize: 15
  }
});
