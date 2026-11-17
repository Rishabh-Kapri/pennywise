# Improvements

Things to make smarter later. Each entry: what's wrong today, what we want.

## Statements: take any file, decide on our side

**Today.** The statement pipeline's Read statement step has one reader per budget. Bank export reads CSVs only; GLM-OCR and Your AI read PDFs only. So the budget has to pick one kind of file:

- With Bank export picked, emailed statements (always PDFs) fail, so From email has to be turned off and every statement uploaded by hand.
- With a PDF reader picked, an uploaded CSV fails.
- Excel exports (`.xls`, `.xlsx`), which many banks offer, can't be read at all.

**Want.** Take PDF, CSV and XLS/XLSX, from uploads and from email, and choose the reader per file on our side. The user shouldn't have to match the setting to the file.

- Detect the file type from its bytes, not only the name (`%PDF-`, the XLSX zip and the old XLS binary signatures, else text).
- Route by type: CSV and Excel to a parser like `bankexport` (Excel read into the same records, so column finding and card summary parsing are shared); PDF to the PDF reader the budget chose.
- The pipeline setting then picks the PDF reader only, and From email can stay on.
- Keep the line/row number for spreadsheets (`StatementRow.line`) so the viewer can point at it; an Excel viewer needs the sheet too.
- Upload's file picker and the API's allowed types gain `.xls`/`.xlsx`.
