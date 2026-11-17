/**
 * Splits a bank export into trimmed fields, numbered the way the reader
 * numbers them (StatementRow.line): HDFC's "~|~" export line by line, empty
 * lines included; a CSV by record, empty lines skipped as Go's encoding/csv
 * does, a quoted field may span lines.
 */
export function parseExport(raw: string): string[][] {
  const text = raw.charCodeAt(0) === 0xfeff ? raw.slice(1) : raw;
  if (text.includes('~|~')) {
    const lines = text.split('\n');
    // a final newline isn't a line
    if (lines.at(-1) === '') lines.pop();
    return lines.map((line) => line.split('~|~').map((f) => f.trim()));
  }
  const records: string[][] = [];
  let i = 0;
  const n = text.length;
  const atLineEnd = () => text[i] === '\n' || (text[i] === '\r' && text[i + 1] === '\n');
  const skipLineEnd = () => (i += text[i] === '\r' ? 2 : 1);
  while (i < n) {
    if (atLineEnd()) {
      skipLineEnd();
      continue;
    }
    const record: string[] = [];
    for (;;) {
      let field = '';
      if (text[i] === '"') {
        i++;
        while (i < n) {
          if (text[i] === '"') {
            if (text[i + 1] === '"') {
              field += '"';
              i += 2;
              continue;
            }
            i++;
            break;
          }
          field += text[i++];
        }
      }
      while (i < n && text[i] !== ',' && !atLineEnd()) field += text[i++];
      record.push(field.trim());
      if (i < n && text[i] === ',') {
        i++;
        continue;
      }
      if (i < n) skipLineEnd();
      break;
    }
    records.push(record);
  }
  return records;
}
