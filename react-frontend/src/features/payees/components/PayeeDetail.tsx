import { useEffect, useMemo, useState } from 'react';
import {
  ArrowLeft,
  ArrowRight,
  Check,
  GitMerge,
  ListMagnifyingGlass,
  MagnifyingGlass,
  PencilSimpleLine,
  Plus,
  Trash,
  Warning,
  X,
} from '@phosphor-icons/react';
import { formatBudgetAmount } from '@/features/category/components/budgetFormat';
import { getLocaleDate } from '@/utils/date.utils';
import type { Payee, PayeeRule, PayeeStats } from '../types/payee.types';
import type { PayeeRuleInput } from '../store/payeeSlice';
import { formatLastUsed } from '../payeeUtils';
import { PayeeAvatar } from './PayeeAvatar';
import { RuleEditor } from './RuleEditor';
import styles from './Payees.module.css';

interface PayeeDetailProps {
  payee: Payee;
  stats?: PayeeStats;
  rules: PayeeRule[];
  allRules: PayeeRule[];
  payees: Payee[];
  categoryNameById: Record<string, string>;
  saving: boolean;
  onBack: () => void;
  onRename: (name: string) => Promise<boolean>;
  onDelete: () => void;
  onMerge: () => void;
  onViewTransactions: () => void;
  onSaveRule: (input: PayeeRuleInput) => Promise<boolean>;
  onDeleteRule: (rule: PayeeRule) => void;
}

export function PayeeDetail({
  payee,
  stats,
  rules,
  allRules,
  payees,
  categoryNameById,
  saving,
  onBack,
  onRename,
  onDelete,
  onMerge,
  onViewTransactions,
  onSaveRule,
  onDeleteRule,
}: PayeeDetailProps) {
  const isTransfer = Boolean(payee.transferAccountId);
  const [isRenaming, setIsRenaming] = useState(false);
  const [nameDraft, setNameDraft] = useState(payee.name);
  // 'new' for the add form, a rule id while editing that rule
  const [editing, setEditing] = useState<string | null>(null);
  const [ruleQuery, setRuleQuery] = useState('');

  useEffect(() => {
    setIsRenaming(false);
    setNameDraft(payee.name);
    setEditing(null);
    setRuleQuery('');
  }, [payee.id, payee.name]);

  const visibleRules = useMemo(() => {
    const query = ruleQuery.trim().toLowerCase();
    if (!query) return rules;
    return rules.filter(
      (rule) => rule.matchString.toLowerCase().includes(query) || rule.categoryName?.toLowerCase().includes(query),
    );
  }, [rules, ruleQuery]);

  const submitRename = async () => {
    const name = nameDraft.trim();
    if (!name || name === payee.name) {
      setIsRenaming(false);
      setNameDraft(payee.name);
      return;
    }
    if (await onRename(name)) setIsRenaming(false);
  };

  const saveRule = async (input: PayeeRuleInput) => {
    if (await onSaveRule(input)) setEditing(null);
  };

  const count = stats?.transactionCount ?? 0;
  const topCategory = stats?.topCategoryId ? categoryNameById[stats.topCategoryId] : undefined;
  const inactiveRules = rules.filter((rule) => !rule.categoryId).length;

  return (
    <div className={styles.detail}>
      <button type="button" className={styles.backButton} onClick={onBack}>
        <ArrowLeft size={16} /> All payees
      </button>

      <header className={styles.detailHeader}>
        <PayeeAvatar name={payee.name} transfer={isTransfer} size="lg" />
        <div className={styles.detailTitle}>
          {isRenaming ? (
            <form
              className={styles.renameForm}
              onSubmit={(event) => {
                event.preventDefault();
                submitRename();
              }}>
              <input
                className={styles.renameInput}
                value={nameDraft}
                autoFocus
                aria-label="Payee name"
                onChange={(event) => setNameDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Escape') {
                    event.stopPropagation();
                    setIsRenaming(false);
                    setNameDraft(payee.name);
                  }
                }}
              />
              <button type="submit" className={styles.iconButton} aria-label="Save name" disabled={saving}>
                <Check size={16} />
              </button>
              <button
                type="button"
                className={styles.iconButton}
                aria-label="Cancel rename"
                onClick={() => {
                  setIsRenaming(false);
                  setNameDraft(payee.name);
                }}>
                <X size={16} />
              </button>
            </form>
          ) : (
            <h2>
              {payee.name}
              {!isTransfer && (
                <button
                  type="button"
                  className={styles.inlineEdit}
                  aria-label="Rename payee"
                  onClick={() => setIsRenaming(true)}>
                  <PencilSimpleLine size={16} />
                </button>
              )}
            </h2>
          )}
          <p>
            {isTransfer
              ? 'Transfer payee. It belongs to an account, so it can’t be renamed, merged or deleted here.'
              : payee.createdAt && !payee.createdAt.startsWith('0001')
                ? `Added ${getLocaleDate(payee.createdAt, { day: 'numeric', month: 'short', year: 'numeric' })}`
                : 'Payee'}
          </p>
        </div>
        <div className={styles.detailActions}>
          <button type="button" className={styles.secondaryButton} onClick={onViewTransactions} disabled={count === 0}>
            <ListMagnifyingGlass size={16} /> Transactions
          </button>
          {!isTransfer && (
            <>
              <button type="button" className={styles.secondaryButton} onClick={onMerge}>
                <GitMerge size={16} /> Merge
              </button>
              <button type="button" className={styles.dangerIconButton} aria-label="Delete payee" onClick={onDelete}>
                <Trash size={16} />
              </button>
            </>
          )}
        </div>
      </header>

      <dl className={styles.statGrid}>
        <div>
          <dt>Transactions</dt>
          <dd>{count.toLocaleString('en-IN')}</dd>
        </div>
        <div>
          <dt>Spent</dt>
          <dd>{formatBudgetAmount(stats?.totalOutflow ?? 0)}</dd>
        </div>
        {(stats?.totalInflow ?? 0) > 0 && (
          <div>
            <dt>Received</dt>
            <dd className={styles.positive}>{formatBudgetAmount(stats!.totalInflow)}</dd>
          </div>
        )}
        <div>
          <dt>Last used</dt>
          <dd>{formatLastUsed(stats?.lastUsed) ?? '—'}</dd>
        </div>
        <div>
          <dt>Usually</dt>
          <dd title={topCategory}>{topCategory ?? '—'}</dd>
        </div>
      </dl>

      {!isTransfer && (
        <section className={styles.rulesSection}>
          <div className={styles.sectionHeader}>
            <div>
              <h3>
                Rules <span className={styles.sectionCount}>{rules.length}</span>
              </h3>
              <p>When a bank email’s merchant or UPI handle matches, the transaction gets this payee and category.</p>
            </div>
            {editing !== 'new' && (
              <button type="button" className={styles.primaryButton} onClick={() => setEditing('new')}>
                <Plus size={15} weight="bold" /> Add rule
              </button>
            )}
          </div>

          {inactiveRules > 0 && (
            <div className={styles.notice}>
              <Warning size={15} />
              {inactiveRules === 1 ? '1 rule has' : `${inactiveRules} rules have`} no category, so the email pipeline
              skips {inactiveRules === 1 ? 'it' : 'them'}.
            </div>
          )}

          {editing === 'new' && (
            <RuleEditor
              payeeId={payee.id}
              payees={payees}
              allRules={allRules}
              categoryNameById={categoryNameById}
              saving={saving}
              onSave={saveRule}
              onCancel={() => setEditing(null)}
            />
          )}

          {rules.length > 5 && (
            <label className={styles.inlineSearch}>
              <MagnifyingGlass size={14} />
              <input
                value={ruleQuery}
                placeholder={`Filter ${rules.length} rules`}
                onChange={(event) => setRuleQuery(event.target.value)}
              />
            </label>
          )}

          <div className={styles.ruleCards}>
            {visibleRules.map((rule) =>
              editing === rule.id ? (
                <RuleEditor
                  key={rule.id}
                  rule={rule}
                  payeeId={payee.id}
                  allowPayeeChange
                  payees={payees}
                  allRules={allRules}
                  categoryNameById={categoryNameById}
                  saving={saving}
                  onSave={saveRule}
                  onCancel={() => setEditing(null)}
                />
              ) : (
                <div key={rule.id} className={styles.ruleCard}>
                  <span className={rule.matchType === 'PATTERN' ? styles.typePattern : styles.typeExact}>
                    {rule.matchType === 'PATTERN' ? 'Pattern' : 'Exact'}
                  </span>
                  <code className={styles.ruleString} title={rule.matchString}>
                    {rule.matchString}
                  </code>
                  <ArrowRight size={14} className={styles.ruleArrow} />
                  {rule.categoryId ? (
                    <span className={styles.categoryChip}>{rule.categoryName ?? categoryNameById[rule.categoryId]}</span>
                  ) : (
                    <span className={styles.categoryMissing}>
                      <Warning size={13} /> No category
                    </span>
                  )}
                  <div className={styles.ruleCardActions}>
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
                  </div>
                </div>
              ),
            )}
            {rules.length === 0 && editing !== 'new' && (
              <div className={styles.emptyRules}>
                <p>No rules yet.</p>
                <span>
                  Add the merchant name or UPI handle from this payee’s bank emails, and future emails are categorised
                  without AI.
                </span>
              </div>
            )}
            {rules.length > 0 && visibleRules.length === 0 && (
              <div className={styles.listEmpty}>No rules match “{ruleQuery.trim()}”.</div>
            )}
          </div>
        </section>
      )}
    </div>
  );
}
