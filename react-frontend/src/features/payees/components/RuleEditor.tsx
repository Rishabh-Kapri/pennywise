import { useMemo, useState } from 'react';
import { CheckCircle, Info, Warning, XCircle } from '@phosphor-icons/react';
import { CategoryDropdown } from '@/features/transactions/components/popovers/CategoryPopover';
import { SearchSelect } from '@/features/transactions/components/popovers/SearchSelect';
import type { Payee, PayeeMatchType, PayeeRule } from '../types/payee.types';
import type { PayeeRuleInput } from '../store/payeeSlice';
import { hasWildcard, ruleMatches } from '../payeeUtils';
import styles from './Payees.module.css';

interface RuleEditorProps {
  /** the rule being edited; absent when adding */
  rule?: PayeeRule;
  /** fixed payee for a new rule; when absent the editor asks for one */
  payeeId?: string;
  /** show the payee picker even with a payee (editing: move the rule) */
  allowPayeeChange?: boolean;
  payees: Payee[];
  allRules: PayeeRule[];
  categoryNameById: Record<string, string>;
  saving: boolean;
  onSave: (input: PayeeRuleInput) => void;
  onCancel: () => void;
}

export function RuleEditor({
  rule,
  payeeId: fixedPayeeId,
  allowPayeeChange,
  payees,
  allRules,
  categoryNameById,
  saving,
  onSave,
  onCancel,
}: RuleEditorProps) {
  const [matchString, setMatchString] = useState(rule?.matchString ?? '');
  const [matchType, setMatchType] = useState<PayeeMatchType>(rule?.matchType ?? 'EXACT');
  const [categoryId, setCategoryId] = useState(rule?.categoryId ?? '');
  const [payeeId, setPayeeId] = useState(rule?.payeeId ?? fixedPayeeId ?? '');
  const [sample, setSample] = useState('');

  const showPayeeField = allowPayeeChange || !fixedPayeeId;
  const trimmed = matchString.trim();
  const payeeItems = useMemo(
    () =>
      payees
        .filter((payee) => payee.id && !payee.transferAccountId)
        .map((payee) => ({ id: payee.id!, name: payee.name }))
        .sort((a, b) => a.name.localeCompare(b.name)),
    [payees],
  );
  const payeeName = payees.find((payee) => payee.id === payeeId)?.name ?? '';

  // match strings are unique per budget; the API upserts on them
  const conflict = useMemo(
    () => (trimmed ? allRules.find((other) => other.id !== rule?.id && other.matchString === trimmed) : undefined),
    [allRules, rule?.id, trimmed],
  );
  const isMove = Boolean(rule && payeeId && payeeId !== rule.payeeId);
  // a PATCH that collides with another rule's string fails on the unique key
  const blockingConflict = Boolean(conflict && rule && !isMove);
  const canSave = Boolean(trimmed && payeeId && !blockingConflict && !saving);

  const submit = () => {
    if (!canSave) return;
    onSave({
      ruleId: rule?.id,
      originalPayeeId: rule?.payeeId,
      originalMatchString: rule?.matchString,
      payeeId,
      matchString: trimmed,
      matchType,
      categoryId,
    });
  };

  const sampleMatches = sample && trimmed ? ruleMatches({ matchString: trimmed, matchType } as PayeeRule, sample) : null;

  return (
    <form
      className={styles.ruleEditor}
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape') {
          event.stopPropagation();
          onCancel();
        }
      }}>
      <div className={styles.ruleEditorGrid}>
        <label className={styles.field}>
          <span>Bank string</span>
          <input
            className={`${styles.input} ${styles.mono}`}
            value={matchString}
            autoFocus
            spellCheck={false}
            placeholder={matchType === 'PATTERN' ? 'SWIGGY%' : 'swiggy@axisbank'}
            onChange={(event) => setMatchString(event.target.value)}
          />
        </label>

        <div className={styles.field}>
          <span>Match</span>
          <div className={styles.segmented} role="radiogroup" aria-label="Match type">
            {(['EXACT', 'PATTERN'] as const).map((type) => (
              <button
                key={type}
                type="button"
                role="radio"
                aria-checked={matchType === type}
                className={matchType === type ? styles.segmentActive : ''}
                onClick={() => setMatchType(type)}>
                {type === 'EXACT' ? 'Exact' : 'Pattern'}
              </button>
            ))}
          </div>
        </div>

        {showPayeeField && (
          <div className={styles.field}>
            <span>{rule ? 'Payee' : 'Assign to payee'}</span>
            <SearchSelect
              value={payeeName}
              items={payeeItems}
              variant="form"
              placeholder="Choose a payee"
              emptyText="No payees match"
              onSelect={(item) => setPayeeId(item.id)}
            />
          </div>
        )}

        <div className={styles.field}>
          <span>Category</span>
          <CategoryDropdown
            variant="form"
            value={categoryId ? (categoryNameById[categoryId] ?? '') : ''}
            placeholder="No category"
            onClick={(id) => setCategoryId(id)}
          />
        </div>
      </div>

      <div className={styles.ruleHints}>
        {matchType === 'PATTERN' ? (
          <p>
            <Info size={14} />
            <span>
              Case-insensitive. <code>%</code> matches any run of characters, <code>_</code> exactly one.
              {trimmed && !hasWildcard(trimmed) && ' No wildcard yet, so this only matches the whole string.'}
            </span>
          </p>
        ) : (
          <p>
            <Info size={14} />
            <span>Matches when the merchant name or UPI handle is exactly this, including case.</span>
          </p>
        )}
        {!categoryId && (
          <p className={styles.hintWarn}>
            <Warning size={14} />
            <span>Without a category the email pipeline skips this rule and falls back to AI.</span>
          </p>
        )}
        {conflict && (
          <p className={blockingConflict ? styles.hintError : styles.hintWarn}>
            <Warning size={14} />
            <span>
              {blockingConflict
                ? `Another rule, for ${conflict.payeeName ?? 'another payee'}, already uses this string.`
                : `Already a rule for ${conflict.payeeName ?? 'another payee'}; saving moves it here.`}
            </span>
          </p>
        )}
      </div>

      {matchType === 'PATTERN' && (
        <div className={styles.sampleRow}>
          <input
            className={`${styles.input} ${styles.mono}`}
            value={sample}
            spellCheck={false}
            placeholder="Try it on a bank string"
            onChange={(event) => setSample(event.target.value)}
          />
          {sampleMatches !== null &&
            (sampleMatches ? (
              <span className={styles.sampleYes}>
                <CheckCircle size={16} weight="fill" /> Matches
              </span>
            ) : (
              <span className={styles.sampleNo}>
                <XCircle size={16} weight="fill" /> No match
              </span>
            ))}
        </div>
      )}

      <div className={styles.editorActions}>
        <button type="button" className={styles.ghostButton} onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className={styles.primaryButton} disabled={!canSave}>
          {saving ? 'Saving…' : rule ? (isMove ? 'Save and move' : 'Save rule') : 'Add rule'}
        </button>
      </div>
    </form>
  );
}
