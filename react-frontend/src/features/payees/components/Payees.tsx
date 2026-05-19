import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { useHeader } from '@/context/HeaderContext';
import { fetchAllPayees } from '@/features/payees/store';
import type { Payee, PayeeRule } from '@/features/payees/types/payee.types';
import { apiClient, LoadingState, toast } from '@/utils';
import { PencilSimpleLine as Edit2, Plus, FloppyDisk as Save, MagnifyingGlass as Search, Tag as Tags, Trash as Trash2, Users as UsersRound, X } from '@phosphor-icons/react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import styles from './Payees.module.css';

type RuleForm = {
  id?: string;
  matchString: string;
  matchType: string;
  categoryId: string;
};

const emptyRuleForm: RuleForm = {
  matchString: '',
  matchType: 'EXACT',
  categoryId: '',
};

function getPayeeId(payee: Payee) {
  return payee.id ?? '';
}

export default function Payees() {
  const dispatch = useAppDispatch();
  const { setHeaderContent } = useHeader();
  const { allPayees, loading, error } = useAppSelector((state) => state.payees);
  const categoryGroups = useAppSelector((state) => state.categories.allCategoryGroups);
  const [selectedPayeeId, setSelectedPayeeId] = useState<string>('');
  const [searchTerm, setSearchTerm] = useState('');
  const [rules, setRules] = useState<PayeeRule[]>([]);
  const [rulesLoading, setRulesLoading] = useState<LoadingState>(LoadingState.IDLE);
  const [rulesError, setRulesError] = useState<string | null>(null);
  const [newPayeeName, setNewPayeeName] = useState('');
  const [editingPayeeId, setEditingPayeeId] = useState<string | null>(null);
  const [payeeNameDraft, setPayeeNameDraft] = useState('');
  const [ruleForm, setRuleForm] = useState<RuleForm>(emptyRuleForm);
  const [isSaving, setIsSaving] = useState(false);

  const categoryOptions = useMemo(
    () => categoryGroups.flatMap((group) => group.categories ?? []),
    [categoryGroups],
  );

  const fetchRules = useCallback((payeeId: string) => {
    if (!payeeId) {
      setRules([]);
      return;
    }

    setRulesLoading(LoadingState.PENDING);
    setRulesError(null);
    apiClient
      .get<PayeeRule[]>(`payees/${payeeId}/rules`)
      .then((res) => {
        setRules(res ?? []);
        setRulesLoading(LoadingState.SUCCESS);
      })
      .catch((err: Error) => {
        setRules([]);
        setRulesError(err.message || 'Failed to load payee rules');
        setRulesLoading(LoadingState.ERROR);
      });
  }, []);

  useEffect(() => {
    setHeaderContent(null);
    dispatch(fetchAllPayees());
  }, [dispatch, setHeaderContent]);

  const filteredPayees = useMemo(() => {
    const query = searchTerm.trim().toLowerCase();
    return allPayees
      .filter((payee) => !query || payee.name.toLowerCase().includes(query))
      .sort((a, b) => a.name.localeCompare(b.name))

  }, [allPayees, searchTerm]);

  const selectedPayee = useMemo(
    () => allPayees.find((payee) => getPayeeId(payee) === selectedPayeeId) ?? null,
    [allPayees, selectedPayeeId],
  );

  useEffect(() => {
    fetchRules(selectedPayeeId);
    setRuleForm(emptyRuleForm);
  }, [fetchRules, selectedPayeeId]);

  const refreshPayees = async () => {
    await dispatch(fetchAllPayees()).unwrap();
  };

  const handleCreatePayee = async () => {
    const name = newPayeeName.trim();
    if (!name) return;
    setIsSaving(true);
    try {
      const created = await apiClient.post<Payee>('payees', { name } as Partial<Payee>);
      await refreshPayees();
      setSelectedPayeeId(created.id ?? selectedPayeeId);
      setNewPayeeName('');
      toast.success('Payee added');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to add payee');
    } finally {
      setIsSaving(false);
    }
  };

  const startEditingPayee = (payee: Payee) => {
    setEditingPayeeId(getPayeeId(payee));
    setPayeeNameDraft(payee.name);
  };

  const handleUpdatePayee = async (payeeId: string) => {
    const name = payeeNameDraft.trim();
    if (!name) return;
    setIsSaving(true);
    try {
      await apiClient.patch<Payee>(`payees/${payeeId}`, { name } as Partial<Payee>);
      await refreshPayees();
      setEditingPayeeId(null);
      setPayeeNameDraft('');
      toast.success('Payee updated');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update payee');
    } finally {
      setIsSaving(false);
    }
  };

  const handleDeletePayee = async (payeeId: string) => {
    if (!window.confirm('Delete this payee? Existing transactions may still reference it.')) return;
    setIsSaving(true);
    try {
      await apiClient.delete(`payees/${payeeId}`);
      await refreshPayees();
      if (selectedPayeeId === payeeId) {
        setSelectedPayeeId('');
      }
      toast.success('Payee deleted');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete payee');
    } finally {
      setIsSaving(false);
    }
  };

  const handleSaveRule = async () => {
    if (!selectedPayeeId || !ruleForm.matchString.trim()) return;
    setIsSaving(true);
    const payload = {
      matchString: ruleForm.matchString.trim(),
      matchType: ruleForm.matchType,
      categoryId: ruleForm.categoryId || undefined,
    } as Partial<PayeeRule>;
    try {
      if (ruleForm.id) {
        await apiClient.patch<PayeeRule>(`payees/${selectedPayeeId}/rules/${ruleForm.id}`, payload);
        toast.success('Rule updated');
      } else {
        await apiClient.post<PayeeRule>(`payees/${selectedPayeeId}/rules`, payload);
        toast.success('Rule added');
      }
      setRuleForm(emptyRuleForm);
      fetchRules(selectedPayeeId);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save rule');
    } finally {
      setIsSaving(false);
    }
  };

  const handleEditRule = (rule: PayeeRule) => {
    setRuleForm({
      id: rule.id,
      matchString: rule.matchString,
      matchType: rule.matchType || 'EXACT',
      categoryId: rule.categoryId ?? '',
    });
  };

  const handleDeleteRule = async (ruleId: string) => {
    if (!selectedPayeeId || !window.confirm('Delete this payee rule?')) return;
    setIsSaving(true);
    try {
      await apiClient.delete(`payees/${selectedPayeeId}/rules/${ruleId}`);
      fetchRules(selectedPayeeId);
      if (ruleForm.id === ruleId) setRuleForm(emptyRuleForm);
      toast.success('Rule deleted');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete rule');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <section className={styles.page}>
      <div className={styles.heading}>
        <div>
          <span className={styles.kicker}>Payee rules</span>
          <h1>Payees</h1>
          <p>Review saved payees, their bank match strings, and default categories.</p>
        </div>
        <div className={styles.countBadge}>{allPayees.length} payees</div>
      </div>

      <div className={styles.contentGrid}>
        <div className={styles.listPanel}>
          <div className={styles.addPayeeForm}>
            <input
              type="text"
              value={newPayeeName}
              placeholder="New payee name"
              onChange={(event) => setNewPayeeName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') handleCreatePayee();
              }}
            />
            <button type="button" className={styles.primaryButton} onClick={handleCreatePayee} disabled={isSaving}>
              <Plus size={16} />
              Add
            </button>
          </div>

          <label className={styles.searchBox}>
            <Search size={16} />
            <input
              type="text"
              value={searchTerm}
              placeholder="Search payees"
              onChange={(event) => setSearchTerm(event.target.value)}
            />
          </label>

          {loading === LoadingState.PENDING && <div className={styles.emptyState}>Loading payees...</div>}
          {loading === LoadingState.ERROR && <div className={styles.errorState}>{error}</div>}
          {loading !== LoadingState.PENDING && filteredPayees.length === 0 && (
            <div className={styles.emptyState}>No payees found.</div>
          )}

          <div className={styles.payeeList}>
            {filteredPayees.map((payee) => {
              const payeeId = getPayeeId(payee);
              const isSelected = payeeId === selectedPayeeId;
              const isEditing = payeeId === editingPayeeId;
              return (
                <div key={payeeId} className={`${styles.payeeItem} ${isSelected ? styles.payeeItemSelected : ''}`}>
                  <button type="button" className={styles.payeeSelectButton} onClick={() => setSelectedPayeeId(payeeId)}>
                    <span className={styles.payeeIcon}>
                      <UsersRound size={16} weight={isSelected ? 'fill' : 'regular'} />
                    </span>
                    {isEditing ? (
                      <input
                        type="text"
                        value={payeeNameDraft}
                        onClick={(event) => event.stopPropagation()}
                        onChange={(event) => setPayeeNameDraft(event.target.value)}
                      />
                    ) : (
                      <span>{payee.name}</span>
                    )}
                  </button>
                  <div className={styles.rowActions}>
                    {isEditing ? (
                      <>
                        <button type="button" aria-label="Save payee" onClick={() => handleUpdatePayee(payeeId)}>
                          <Save size={15} />
                        </button>
                        <button type="button" aria-label="Cancel payee edit" onClick={() => setEditingPayeeId(null)}>
                          <X size={15} />
                        </button>
                      </>
                    ) : (
                      <>
                        <button
                          type="button"
                          className={styles.editButton}
                          aria-label="Edit payee"
                          onClick={() => startEditingPayee(payee)}>
                          <Edit2 size={15} />
                        </button>
                        <button
                          type="button"
                          className={styles.deleteButton}
                          aria-label="Delete payee"
                          onClick={() => handleDeletePayee(payeeId)}>
                          <Trash2 size={15} />
                        </button>
                      </>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <aside className={styles.detailPanel}>
          {selectedPayee ? (
            <>
              <div className={styles.detailHeader}>
                <div className={styles.detailIcon}>
                  <Tags size={20} />
                </div>
                <div>
                  <span className={styles.kicker}>Selected payee</span>
                  <h2>{selectedPayee.name}</h2>
                </div>
              </div>

              <div className={styles.ruleForm}>
                <input
                  type="text"
                  value={ruleForm.matchString}
                  placeholder="Match string"
                  onChange={(event) => setRuleForm((prev) => ({ ...prev, matchString: event.target.value }))}
                />
                <select
                  value={ruleForm.matchType}
                  onChange={(event) => setRuleForm((prev) => ({ ...prev, matchType: event.target.value }))}>
                  <option value="EXACT">Exact</option>
                  <option value="PATTERN">Pattern</option>
                </select>
                <select
                  value={ruleForm.categoryId}
                  onChange={(event) => setRuleForm((prev) => ({ ...prev, categoryId: event.target.value }))}>
                  <option value="">No default category</option>
                  {categoryOptions.map((category) => (
                    <option key={category.id} value={category.id ?? ''}>
                      {category.name}
                    </option>
                  ))}
                </select>
                <button type="button" className={styles.primaryButton} onClick={handleSaveRule} disabled={isSaving}>
                  {ruleForm.id ? <Save size={16} /> : <Plus size={16} />}
                  {ruleForm.id ? 'Save rule' : 'Add rule'}
                </button>
                {ruleForm.id && (
                  <button type="button" className={styles.secondaryButton} onClick={() => setRuleForm(emptyRuleForm)}>
                    Cancel
                  </button>
                )}
              </div>

              <div className={styles.summaryCard}>
                <span>Match strings</span>
                <strong>{rules.length}</strong>
              </div>

              {rulesLoading === LoadingState.PENDING && <div className={styles.emptyState}>Loading rules...</div>}
              {rulesLoading === LoadingState.ERROR && <div className={styles.errorState}>{rulesError}</div>}
              {rulesLoading !== LoadingState.PENDING && rules.length === 0 && (
                <div className={styles.emptyState}>No payee rules saved for this payee.</div>
              )}

              <div className={styles.rulesList}>
                {rules.map((rule) => (
                  <div key={rule.id} className={styles.ruleRow}>
                    <div className={styles.ruleMatch}>
                      <span className={styles.ruleType}>{rule.matchType}</span>
                      <strong>{rule.matchString}</strong>
                    </div>
                    <div className={styles.ruleCategory}>
                      <span>Default category</span>
                      <strong>{rule.categoryName ?? 'Uncategorized'}</strong>
                    </div>
                    <div className={styles.rowActions}>
                      <button
                        type="button"
                        className={styles.editButton}
                        aria-label="Edit rule"
                        onClick={() => handleEditRule(rule)}>
                        <Edit2 size={15} />
                      </button>
                      <button
                        type="button"
                        className={styles.deleteButton}
                        aria-label="Delete rule"
                        onClick={() => handleDeleteRule(rule.id)}>
                        <Trash2 size={15} />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className={styles.emptyDetail}>Select a payee to view match strings.</div>
          )}
        </aside>
      </div>
    </section>
  );
}
