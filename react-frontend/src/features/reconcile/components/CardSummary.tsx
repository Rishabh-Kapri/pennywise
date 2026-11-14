import type { StatementSectionReconciliation } from '../types/reconcile.types';
import styles from './StatementReconcile.module.css';

const money = (value: number) =>
  new Intl.NumberFormat('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);
const points = (value: number) => new Intl.NumberFormat('en-IN').format(value);

function longDay(date: string): string {
  const d = new Date(`${date}T00:00:00`);
  return Number.isNaN(d.getTime())
    ? date
    : d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
}

const CHARGE_LABELS: Record<string, string> = {
  interest: 'Interest',
  fee: 'Fees',
  tax: 'GST and taxes',
};

/**
 * What a card statement says beyond its rows: what's due, the charges, reward
 * points and running EMIs. Shows only what the statement stated.
 */
export function CardSummary({ section }: { section: StatementSectionReconciliation }) {
  const tiles = [
    section.totalDue !== undefined && { label: 'Total due', value: money(section.totalDue) },
    section.minimumDue !== undefined && { label: 'Minimum due', value: money(section.minimumDue) },
    section.dueDate && { label: 'Pay by', value: longDay(section.dueDate) },
    section.creditLimit !== undefined && { label: 'Credit limit', value: money(section.creditLimit) },
  ].filter(Boolean) as { label: string; value: string }[];

  const charges = section.charges ?? [];
  const chargesTotal = charges.reduce((sum, c) => sum + c.amount, 0);
  const rewards = section.rewards;
  const rewardRows = rewards
    ? ([
        ['Opening', rewards.opening],
        ['Earned', rewards.earned],
        ['Redeemed', rewards.redeemed],
        ['Expired', rewards.expired],
        ['Closing', rewards.closing],
      ].filter(([, v]) => v !== undefined) as [string, number][])
    : [];
  const plans = section.emiPlans ?? [];

  if (!tiles.length && !charges.length && !rewardRows.length && section.rewardPointsOnRows === undefined && !plans.length) {
    return null;
  }

  return (
    <div className={styles.cardSummary}>
      {tiles.length > 0 && (
        <div className={styles.statTiles}>
          {tiles.map((t) => (
            <div key={t.label} className={styles.statTile}>
              <span>{t.label}</span>
              <strong>{t.value}</strong>
            </div>
          ))}
        </div>
      )}

      <div className={styles.cardPanels}>
        {charges.length > 0 && (
          <section className={styles.cardPanel}>
            <header>
              <h4>Charges</h4>
              <strong>{money(chargesTotal)}</strong>
            </header>
            <ul>
              {charges.map((c) => (
                <li key={c.kind}>
                  <span>
                    {CHARGE_LABELS[c.kind] ?? c.kind}
                    <small>
                      {c.count} line{c.count === 1 ? '' : 's'}
                    </small>
                  </span>
                  <span>{money(c.amount)}</span>
                </li>
              ))}
            </ul>
          </section>
        )}

        {(rewardRows.length > 0 || section.rewardPointsOnRows !== undefined) && (
          <section className={styles.cardPanel}>
            <header>
              <h4>Reward points</h4>
              {rewards?.closing !== undefined && <strong>{points(rewards.closing)}</strong>}
            </header>
            <ul>
              {rewardRows.map(([label, value]) => (
                <li key={label}>
                  <span>{label}</span>
                  <span>{points(value)}</span>
                </li>
              ))}
              {section.rewardPointsOnRows !== undefined && (
                <li>
                  <span>
                    On this statement's rows
                    <small>added up</small>
                  </span>
                  <span>{points(section.rewardPointsOnRows)}</span>
                </li>
              )}
            </ul>
          </section>
        )}

        {plans.length > 0 && (
          <section className={`${styles.cardPanel} ${styles.cardPanelWide}`}>
            <header>
              <h4>EMIs</h4>
              <strong>
                {money(plans.reduce((sum, p) => sum + (p.outstanding ?? 0), 0))}
                <small> left</small>
              </strong>
            </header>
            <ul>
              {plans.map((p, i) => (
                <li key={`${p.description}-${i}`}>
                  <span>
                    {p.description || `EMI ${i + 1}`}
                    <small>
                      {[
                        p.monthlyAmount !== undefined && `${money(p.monthlyAmount)} a month`,
                        p.remainingMonths !== undefined &&
                          (p.tenureMonths !== undefined
                            ? `${p.remainingMonths} of ${p.tenureMonths} months left`
                            : `${p.remainingMonths} months left`),
                        p.interestRate !== undefined && `${p.interestRate}% a year`,
                      ]
                        .filter(Boolean)
                        .join(' · ')}
                    </small>
                  </span>
                  <span>{p.outstanding !== undefined ? money(p.outstanding) : '—'}</span>
                </li>
              ))}
            </ul>
            <p className={styles.muted}>
              Instalments are listed under "Covered by EMI", not as missing: the purchase is already in your budget.
            </p>
          </section>
        )}
      </div>
    </div>
  );
}
