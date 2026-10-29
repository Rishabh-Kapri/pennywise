import { useMemo, useState } from 'react';
import { ArrowRight, CheckCircle, Flask, PencilSimpleLine, Plus, Trash, Warning, XCircle } from '@phosphor-icons/react';
import { getLocaleDate } from '@/utils/date.utils';
import type { Payee, PayeeRule } from '../types/payee.types';
import type { PayeeRuleInput } from '../store/payeeSlice';
import { testRules } from '../payeeUtils';
import { RuleEditor } from './RuleEditor';
import styles from './Payees.module.css';

type RuleFilter = 'all' | 'EXACT' | 'PATTERN' | 'nocategory';
type RuleSort = 'match' | 'payee' | 'newest';

interface RulesViewProps {
  rules: PayeeRule[];
  payees: Payee[];
  categoryNameById: Record<string, string>;
  search: string;
  loading: boolean;
  saving: boolean;
  onOpenPayee: (payeeId: string) => void;
  onSaveRule: (input: PayeeRuleInput) => Promise<boolean>;
  onDeleteRule: (rule: PayeeRule) => void;
}

export function RulesView({
  rules,
  payees,
  categoryNameById,
  search,
  loading,
  saving,
  onOpenPayee,
  onSaveRule,
  onDeleteRule,
}: RulesViewProps) {
  const [filter, setFilter] = useState<RuleFilter>('all');
  const [sort, setSort] = useState<RuleSort>('match');
  const [editing, setEditing] = useState<string | null>(null);
  const [sample, setSample] = useState('');

  const test = useMemo(() => testRules(rules, sample.trim()), [rules, sample]);
  const matchedIds = useMemo(() => new Set(test.matches.map((rule) => rule.id)), [test]);

  const counts = useMemo(
    () => ({
      all: rules.length,
      EXACT: rules.filter((rule) => rule.matchType === 'EXACT').length,
      PATTERN: rules.filter((rule) => rule.matchType === 'PATTERN').length,
      nocategory: rules.filter((rule) => !rule.categoryId).length,
    }),
    [rules],
  );

  const visible = useMemo(() => {
    const query = search.trim().toLowerCase();
    return rules
      .filter((rule) => {
        if (filter === 'nocategory' && rule.categoryId) return false;
        if ((filter === 'EXACT' || filter === 'PATTERN') && rule.matchType !== filter) return false;
        if (!query) return true;
        return [rule.matchString, rule.payeeName, rule.categoryName].some((value) => value?.toLowerCase().includes(query));
      })
      .sort((a, b) => {
        // rules the tester matched float to the top, the winner first
        if (sample.trim()) {
          const rank = (rule: PayeeRule) => (rule.id === test.winner?.id ? 0 : matchedIds.has(rule.id) ? 1 : 2);
          const diff = rank(a) - rank(b);
          if (diff) return diff;
        }
        if (sort === 'payee') return (a.payeeName ?? '').localeCompare(b.payeeName ?? '') || a.matchString.localeCompare(b.matchString);
        if (sort === 'newest') return (b.createdAt ?? '').localeCompare(a.createdAt ?? '');
        return a.matchString.localeCompare(b.matchString, undefined, { sensitivity: 'base' });
      });
  }, [rules, search, filter, sort, sample, test, matchedIds]);

  const saveRule = async (input: PayeeRuleInput) => {
    if (await onSaveRule(input)) setEditing(null);
  };

  const editorProps = { payees, allRules: rules, categoryNameById, saving, onSave: saveRule, onCancel: () => setEditing(null) };

  return (
    <div className={styles.rulesView}>
      <section className={styles.tester} aria-label="Test a bank string">
        <div className={styles.testerHead}>
          <Flask size={18} />
          <div>
            <h3>Test a bank string</h3>
            <p>Paste a merchant name or UPI handle from a bank email to see which rule catches it.</p>
          </div>
        </div>
        <input
          className={`${styles.input} ${styles.mono}`}
          value={sample}
          spellCheck={false}
          placeholder="e.g. SWIGGY INSTAMART or paytmqr281005@paytm"
          onChange={(event) => setSample(event.target.value)}
        />
        {sample.trim() && (
          <div className={styles.testerResult} aria-live="polite">
            {test.winner ? (
              test.winner.categoryId ? (
                <p className={styles.sampleYes}>
                  <CheckCircle size={16} weight="fill" />
                  <span>
                    <code>{test.winner.matchString}</code> assigns <strong>{test.winner.payeeName}</strong> →{' '}
                    <strong>{test.winner.categoryName ?? categoryNameById[test.winner.categoryId]}</strong>
                    {test.matches.length > 1 && ` (also matched by ${test.matches.length - 1} more)`}
                  </span>
                </p>
              ) : (
                <p className={styles.hintWarn}>
                  <Warning size={16} />
                  <span>
                    <code>{test.winner.matchString}</code> matches, but has no category, so the pipeline skips rules
                    and asks AI.
                  </span>
                </p>
              )
            ) : (
              <p className={styles.sampleNo}>
                <XCircle size={16} weight="fill" />
                <span>No rule matches. The pipeline will fall back to learned matches and AI.</span>
              </p>
            )}
          </div>
        )}
      </section>

      <div className={styles.rulesToolbar}>
        <div className={styles.chips} role="tablist" aria-label="Filter rules">
          {(
            [
              ['all', 'All'],
              ['EXACT', 'Exact'],
              ['PATTERN', 'Pattern'],
              ['nocategory', 'No category'],
            ] as const
          )
            .filter(([id]) => id === 'all' || id === filter || counts[id] > 0)
            .map(([id, label]) => (
              <button
                key={id}
                type="button"
                role="tab"
                aria-selected={filter === id}
                className={`${styles.chip} ${filter === id ? styles.chipActive : ''} ${id === 'nocategory' ? styles.chipWarn : ''}`}
                onClick={() => setFilter(id)}>
                {label}
                <span className={styles.chipCount}>{counts[id]}</span>
              </button>
            ))}
        </div>
        <div className={styles.rulesToolbarEnd}>
          <select
            className={styles.sortSelect}
            value={sort}
            aria-label="Sort rules"
            onChange={(event) => setSort(event.target.value as RuleSort)}>
            <option value="match">Bank string</option>
            <option value="payee">Payee</option>
            <option value="newest">Newest</option>
          </select>
          {editing !== 'new' && (
            <button type="button" className={styles.primaryButton} onClick={() => setEditing('new')}>
              <Plus size={15} weight="bold" /> New rule
            </button>
          )}
        </div>
      </div>

      {editing === 'new' && <RuleEditor {...editorProps} />}

      <div className={styles.rulesTable} role="table" aria-label="Payee rules">
        <div className={styles.rulesHead} role="row">
          <span role="columnheader">Bank string</span>
          <span role="columnheader">Payee</span>
          <span role="columnheader">Category</span>
          <span role="columnheader">Added</span>
          <span role="columnheader" className={styles.srOnly}>
            Actions
          </span>
        </div>
        <div className={styles.rulesBody}>
          {loading &&
            rules.length === 0 &&
            Array.from({ length: 6 }, (_, index) => <div key={index} className={styles.rowSkeleton} />)}
          {visible.map((rule) =>
            editing === rule.id ? (
              <div key={rule.id} className={styles.rulesEditRow}>
                <RuleEditor {...editorProps} rule={rule} payeeId={rule.payeeId} allowPayeeChange />
              </div>
            ) : (
              <div
                key={rule.id}
                role="row"
                className={`${styles.rulesRow} ${rule.id === test.winner?.id ? styles.rulesRowWinner : matchedIds.has(rule.id) ? styles.rulesRowMatched : ''}`}>
                <span role="cell" className={styles.rulesMatch}>
                  <span className={rule.matchType === 'PATTERN' ? styles.typePattern : styles.typeExact}>
                    {rule.matchType === 'PATTERN' ? 'Pattern' : 'Exact'}
                  </span>
                  <code className={styles.ruleString} title={rule.matchString}>
                    {rule.matchString}
                  </code>
                </span>
                <span role="cell" className={styles.rulesPayee}>
                  <ArrowRight size={13} className={styles.ruleArrow} />
                  <button type="button" className={styles.linkButton} onClick={() => onOpenPayee(rule.payeeId)}>
                    {rule.payeeName ?? 'Unknown payee'}
                  </button>
                </span>
                <span role="cell">
                  {rule.categoryId ? (
                    <span className={styles.categoryChip}>{rule.categoryName ?? categoryNameById[rule.categoryId]}</span>
                  ) : (
                    <span className={styles.categoryMissing}>
                      <Warning size={13} /> No category
                    </span>
                  )}
                </span>
                <span role="cell" className={styles.rulesDate}>
                  {rule.createdAt ? getLocaleDate(rule.createdAt, { day: 'numeric', month: 'short', year: 'numeric' }) : '—'}
                </span>
                <span role="cell" className={styles.ruleCardActions}>
                  <button type="button" className={styles.iconButton} aria-label="Edit rule" onClick={() => setEditing(rule.id)}>
                    <PencilSimpleLine size={15} />
                  </button>
                  <button
                    type="button"
                    className={`${styles.iconButton} ${styles.iconDanger}`}
                    aria-label="Delete rule"
                    onClick={() => onDeleteRule(rule)}>
                    <Trash size={15} />
                  </button>
                </span>
              </div>
            ),
          )}
          {!loading && visible.length === 0 && (
            <div className={styles.listEmpty}>
              {rules.length === 0
                ? 'No rules yet. Rules are learned when you correct an email transaction, or add one here.'
                : 'No rules match these filters.'}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
