import { useEffect, useRef, useState } from 'react';
import { sampleCharges, sampleEntries, sampleLoans, samplePrograms } from '../data/sampleStatement';
import styles from './Statements.module.css';

const money = (value: number) => new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR' }).format(value);
const tabs = ['Overview', 'Charges', 'Rewards', 'EMI', 'Review'] as const;
type Tab = typeof tabs[number];

export default function Statements({ accountName }: { accountName: string }) {
  const [sample, setSample] = useState(false);
  const [tab, setTab] = useState<Tab>('Overview');
  const [file, setFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [dragging, setDragging] = useState(false);
  const [query, setQuery] = useState('');
  const [evidence, setEvidence] = useState('All');
  const [reviewed, setReviewed] = useState<string[]>([]);
  const fileInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!file) return;
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  function selectFile(next?: File) {
    if (!next) return;
    if (!/\.pdf$/i.test(next.name) || (next.type && next.type !== 'application/pdf')) {
      setError('Choose a PDF statement.');
      return;
    }
    if (next.size > 20 * 1024 * 1024) {
      setError('Choose a PDF smaller than 20 MB.');
      return;
    }
    if (!next.size) {
      setError('This file is empty. Choose another PDF.');
      return;
    }
    setError('');
    setPreviewUrl(null);
    setFile(next);
  }

  const charges = sampleCharges.filter((charge) =>
    (evidence === 'All' || charge.evidence === evidence)
    && `${charge.name} ${charge.detail} ${charge.source}`.toLowerCase().includes(query.toLowerCase()));
  const earned = sampleEntries.reduce((sum, row) => sum + Math.max(row.points ?? 0, 0), 0);
  const reversed = sampleEntries.reduce((sum, row) => sum + Math.min(row.points ?? 0, 0), 0);

  return (
    <section className={styles.root} aria-label={`Statements for ${accountName}`}>
      <header className={styles.header}>
        <div><p className={styles.eyebrow}>CARD INSIGHTS</p><h2>Statements</h2><p>Understand the charges behind your spending and the rewards you earned.</p></div>
        <button className={styles.primary} type="button" onClick={() => fileInput.current?.click()}>Upload PDF</button>
      </header>
      <input ref={fileInput} type="file" accept=".pdf,application/pdf" className={styles.fileInput} aria-label="Choose credit card statement PDF" onChange={(event) => { selectFile(event.target.files?.[0]); event.target.value = ''; }} />
      {error && <p className={styles.error} role="alert">{error}</p>}
      {file && <div className={styles.notice} role="status">
        <div><strong>{file.name}</strong><p>{(file.size / 1024).toFixed(0)} KB · Selected locally. Upload and extraction are not connected yet; this PDF has not been saved or analysed.</p></div>
        <div className={styles.actions}>{previewUrl && <a href={previewUrl} target="_blank" rel="noreferrer">Open PDF</a>}<button type="button" onClick={() => { setFile(null); setPreviewUrl(null); }}>Remove</button></div>
      </div>}
      {!sample ? <div className={styles.empty}>
        <div className={`${styles.dropZone} ${dragging ? styles.dragging : ''}`} onDragOver={(event) => { event.preventDefault(); setDragging(true); }} onDragLeave={() => setDragging(false)} onDrop={(event) => { event.preventDefault(); setDragging(false); selectFile(event.dataTransfer.files[0]); }}>
          <span className={styles.documentIcon} aria-hidden="true">▤</span>
          <h3>A clearer picture of your card</h3>
          <p>Drop a PDF here or choose a file to preview it locally.</p>
          <button className={styles.primary} type="button" onClick={() => fileInput.current?.click()}>Choose statement</button>
          <small>PDF · up to 20 MB</small>
        </div>
        <div className={styles.samplePrompt}><div><strong>Explore the statement analysis</strong><p>See charges, rewards, and EMI details using made-up example data.</p></div><button type="button" onClick={() => setSample(true)}>View sample analysis →</button></div>
        <p className={styles.muted}>Statement storage and extraction will be available when the backend is connected.</p>
      </div> : <>
        <div className={styles.notice}><div><strong>Sample analysis · 3 Aug–3 Sep 2026 transaction range</strong><p>Made-up example data, not your selected PDF. Billing period and card variant are unconfirmed. Review changes last only while this view is open.</p></div><button type="button" onClick={() => setSample(false)}>Close sample</button></div>
        <nav className={styles.tabs} aria-label="Statement analysis sections">{tabs.map((item) => <button key={item} type="button" aria-current={tab === item ? 'page' : undefined} className={tab === item ? styles.active : ''} onClick={() => setTab(item)}>{item}{item === 'Review' && <span className={styles.count}>{3 - reviewed.length}</span>}</button>)}</nav>
        <div className={styles.panel}>
          {tab === 'Overview' && <>
            <div className={styles.metrics}>
              <Metric label="Total extra charges" value={money(601.9)} detail="Fees, interest, and GST" />
              <Metric label="GST paid" value={money(96.9)} detail="Matches reported GST summary" />
              <Metric label="Net transaction points" value={(earned + reversed).toLocaleString('en-IN')} detail={`${earned.toLocaleString('en-IN')} earned · ${Math.abs(reversed)} reversed`} />
              <Metric label="Program points" value="500" detail="Shown separately; additive status unknown" />
            </div>
            <div className={styles.columns}>
              <article className={styles.card}><h3>Where the extra cost went</h3>{[
                ['EMI interest', 300], ['GST', 96.9], ['Redemption fee', 100], ['Foreign currency markup', 80], ['DCC fees', 25],
              ].map(([label, amount]) => <div key={label} className={styles.breakdown}><div><span>{label}</span><strong>{money(Number(amount))}</strong></div><div className={styles.track}><div style={{ width: `${Number(amount) / 601.9 * 100}%` }} /></div></div>)}<p className={styles.muted}>EMI principal of {money(7500)} is repayment, excluded from extra charges.</p></article>
              <article className={styles.card}><h3>Reconciliation</h3><p className={styles.success}>✓ GST reconciled</p><p>Extracted GST {money(96.9)} matches the statement summary.</p><p className={styles.success}>✓ Program points reconciled</p><p>Program entries sum to the reported 500 points.</p><p className={styles.warning}>{money(60)} GST needs a parent charge</p><p>Three tax entries remain unassigned. Foreign currency allocations are suggestions.</p><button type="button" onClick={() => setTab('Review')}>Review unresolved entries →</button></article>
            </div>
          </>}
          {tab === 'Charges' && <>
            <div className={styles.toolbar}><label>Search charges<input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Merchant, charge, or reference" /></label><label>Evidence<select value={evidence} onChange={(event) => setEvidence(event.target.value)}>{['All', 'Reference match', 'Reported', 'Inferred', 'Unassigned'].map((value) => <option key={value}>{value}</option>)}</select></label></div>
            <p className={styles.muted}>FX markup is allocated across three purchases below; the consolidated fee is counted once. Expand a row to inspect its evidence.</p>
            <div className={styles.tableWrap}><table><caption>Extra charges by transaction or reason</caption><thead><tr><th scope="col">Transaction / reason</th><th scope="col">Fee / interest</th><th scope="col">GST</th><th scope="col">Total</th><th scope="col">Evidence</th></tr></thead><tbody>{charges.map((charge) => <tr key={charge.name}><td><details><summary>{charge.name}</summary><p>{charge.detail}</p><p>{charge.source}</p></details></td><td>{money(charge.fee)}</td><td>{charge.gst ? money(charge.gst) : 'Not assigned'}</td><td><strong>{money(charge.fee + charge.gst)}</strong></td><td><span className={styles.badge}>{charge.evidence}</span></td></tr>)}</tbody></table>{charges.length === 0 && <p>No charges match these filters.</p>}</div>
          </>}
          {tab === 'Rewards' && <>
            <div className={styles.metrics}><Metric label="Earned on transactions" value={earned.toLocaleString('en-IN')} detail="Explicit positive points" /><Metric label="Reversed" value={String(reversed)} detail="Refund-associated reversals" /><Metric label="Net transaction points" value={(earned + reversed).toLocaleString('en-IN')} detail="Earned minus reversals" /><Metric label="Program points" value="500" detail="Kept separate from transaction points" /></div>
            <p className={styles.notice}>Available points balance is not supplied. Blank reward cells mean “not reported”; program points are not added to transaction points without confirmation.</p>
            <details className={styles.card} open><summary>Rewards program breakdown</summary><div className={styles.tableWrap}><table><thead><tr><th scope="col">Program (statement label)</th><th scope="col">Points</th></tr></thead><tbody>{samplePrograms.map((program) => <tr key={program.name}><td>{program.name}</td><td>{program.points}</td></tr>)}</tbody></table></div></details>
            <div className={styles.tableWrap}><table><caption>All supplied transaction entries</caption><thead><tr><th scope="col">Date</th><th scope="col">Description</th><th scope="col">Billed amount</th><th scope="col">Points</th></tr></thead><tbody>{sampleEntries.map((row, index) => <tr key={index}><td>{row.date}<small>{row.time} IST</small></td><td>{row.description}<small>{row.foreignAmount ?? row.section}{row.credit ? ' · Apparent credit; verify PDF legend' : ''}</small></td><td>{money(row.amount)}</td><td>{row.points === null ? 'Not reported' : `${row.points > 0 ? '+' : ''}${row.points}`}</td></tr>)}</tbody></table></div>
          </>}
          {tab === 'EMI' && <>
            <div className={styles.metrics}><Metric label="This cycle’s principal" value={money(7500)} detail="Excluded from extra costs" /><Metric label="This cycle’s interest" value={money(300)} detail="Included in extra costs" /><Metric label="Remaining principal" value={money(15000)} detail="Statement loan snapshots" /><Metric label="Remaining interest" value={money(300)} detail="Reported remaining interest payable" /></div>
            <div className={styles.columns}>{sampleLoans.map((loan) => <article key={loan.id} className={styles.card}><h3>Loan {loan.id}</h3><p className={styles.muted}>Booked 1 Jan 2026 · 6 months · 15% p.a.</p><dl>{[['Original amount', loan.original], ['This cycle’s principal', loan.principal], ['This cycle’s interest', loan.interest], ['Principal outstanding', loan.outstanding], ['Remaining interest payable', loan.remainingInterest]].map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{money(Number(value))}</dd></div>)}<div><dt>Remaining tenure</dt><dd>2 months</dd></div></dl></article>)}</div><p className={styles.muted}>Pre-closure charges may apply according to the statement. No pre-closure cost or loan-specific GST has been estimated.</p>
          </>}
          {tab === 'Review' && <>
            <h3>Unassigned GST</h3><p className={styles.muted}>These amounts are included in the total. Their underlying fees or transactions are unknown. Marking a sample entry reviewed does not assign it or change its evidence.</p>
            {[['10.00', '00000000000000000000001'], ['30.00', '00000000000000000000002'], ['20.00', '00000000000000000000003']].map(([amount, reference]) => <article className={styles.reviewItem} key={reference}><div><strong>{money(Number(amount))} · IGST at stated 18%</strong><p>3 Aug 2026 · Ref {reference}</p></div><label><input type="checkbox" checked={reviewed.includes(reference)} onChange={(event) => setReviewed((previous) => event.target.checked ? [...previous, reference] : previous.filter((id) => id !== reference))} />Reviewed locally</label></article>)}
            <article className={styles.card}><h3>Suggested purchase links</h3><p>The DCC links and FX allocations, are inferred. They still need confirmation from the full statement.</p><button type="button" onClick={() => { setEvidence('Inferred'); setTab('Charges'); }}>Inspect inferred charges →</button></article>
            <article className={styles.card}><h3>Statement details to verify</h3><p>Card variant, credit/debit legend, billing period, balance totals, and whether program points are additional are not established by the excerpts.</p><p className={styles.muted}>Transaction matching and saved corrections will be available when statement processing is connected.</p></article>
          </>}
        </div>
      </>}
    </section>
  );
}

function Metric({ label, value, detail }: { label: string; value: string; detail: string }) {
  return <article className={styles.metric}><span>{label}</span><strong>{value}</strong><small>{detail}</small></article>;
}
