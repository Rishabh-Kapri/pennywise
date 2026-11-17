import type { StatementSectionReconciliation } from '../types/reconcile.types';
import styles from './StatementReconcile.module.css';

const money = (value: number) =>
  new Intl.NumberFormat('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);
const points = (value: number) => new Intl.NumberFormat('en-IN').format(value);

const parseDay = (date: string) => {
  const d = new Date(`${date}T00:00:00`);
  return Number.isNaN(d.getTime()) ? null : d;
};

function longDay(date: string): string {
  const d = parseDay(date);
  return d ? d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' }) : date;
}

/**
 * "today" or "in 6 days" for a due date still ahead; '' once it has passed.
 * A past date says nothing useful: an old statement was paid long ago.
 */
function dueIn(date: string): string {
  const d = parseDay(date);
  if (!d) return '';
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const days = Math.round((d.getTime() - today.getTime()) / 86_400_000);
  if (days < 0) return '';
  return days === 0 ? 'Due today' : `In ${days} day${days === 1 ? '' : 's'}`;
}

/** Rupees large, paise small: the figure the eye lands on. */
function BigMoney({ value }: { value: number }) {
  const [whole, paise] = money(value).split('.');
  return (
    <>
      {whole}
      <span className={styles.paise}>.{paise}</span>
    </>
  );
}

const CHARGE_LABELS: Record<string, string> = {
  interest: 'Interest',
  fee: 'Fees',
  tax: 'GST and taxes',
};

/**
 * What a card statement says beyond its rows: what's due, the charges,
 * cashback, reward points and running EMIs. One surface, set like the
 * statement's own summary: figures in columns, no boxes inside it. Shows
 * only what the statement stated.
 */
export function CardSummary({ section }: { section: StatementSectionReconciliation }) {
  const charges = section.charges ?? [];
  const chargesTotal = charges.reduce((sum, c) => sum + c.amount, 0);
  const cashback = section.cashback;
  const rewards = section.rewards;
  const rewardRows = rewards
    ? ([
        ['Opening', rewards.opening, ''],
        ['Earned', rewards.earned, '+'],
        ['Redeemed', rewards.redeemed, '−'],
        ['Expired', rewards.expired, '−'],
      ].filter(([, v]) => v !== undefined) as [string, number, string][])
    : [];
  const hasRewards =
    rewardRows.length > 0 || rewards?.closing !== undefined || section.rewardPointsOnRows !== undefined;
  const plans = section.emiPlans ?? [];
  const used =
    section.totalDue !== undefined && section.creditLimit ? Math.min(1, section.totalDue / section.creditLimit) : null;
  const hasDues =
    section.totalDue !== undefined ||
    section.minimumDue !== undefined ||
    !!section.dueDate ||
    section.creditLimit !== undefined;
  const hasFigures = !!cashback || charges.length > 0 || hasRewards;

  if (!hasDues && !hasFigures && !plans.length) return null;

  const countdown = section.dueDate ? dueIn(section.dueDate) : '';

  return (
    <div className={styles.cardSummary}>
      {hasDues && (
        <div className={styles.dues}>
          {section.totalDue !== undefined && (
            <div className={styles.duesLead}>
              <span className={styles.figureLabel}>Total due</span>
              <strong>
                <BigMoney value={section.totalDue} />
              </strong>
              {section.minimumDue !== undefined && (
                <span className={styles.figureNote}>Minimum due {money(section.minimumDue)}</span>
              )}
            </div>
          )}
          {section.totalDue === undefined && section.minimumDue !== undefined && (
            <div>
              <span className={styles.figureLabel}>Minimum due</span>
              <strong>{money(section.minimumDue)}</strong>
            </div>
          )}
          {section.dueDate && (
            <div>
              <span className={styles.figureLabel}>Pay by</span>
              <strong>{longDay(section.dueDate)}</strong>
              {countdown && <span className={styles.figureNote}>{countdown}</span>}
            </div>
          )}
          {section.creditLimit !== undefined && (
            <div>
              <span className={styles.figureLabel}>Credit limit</span>
              <strong>{money(section.creditLimit)}</strong>
              {used !== null && (
                <span className={styles.figureNote}>
                  <span className={styles.meter} aria-hidden>
                    <span style={{ width: `${Math.max(used * 100, 2)}%` }} />
                  </span>
                  This bill is {Math.round(used * 100)}% of it
                </span>
              )}
            </div>
          )}
        </div>
      )}

      {hasFigures && (
        <div className={styles.figures}>
          {cashback && (
            <section>
              <header>
                <h4>Cashback</h4>
                <strong className={styles.inflow}>+{money(cashback.amount)}</strong>
              </header>
              <p>
                From {cashback.count} line{cashback.count === 1 ? '' : 's'} on this statement, after reversals.
              </p>
            </section>
          )}

          {charges.length > 0 && (
            <section>
              <header>
                <h4>Charges</h4>
                <strong>{money(chargesTotal)}</strong>
              </header>
              <dl>
                {charges.map((c) => (
                  <div key={c.kind}>
                    <dt>
                      {CHARGE_LABELS[c.kind] ?? c.kind}
                      <small>
                        {c.count} line{c.count === 1 ? '' : 's'}
                      </small>
                    </dt>
                    <dd>{money(c.amount)}</dd>
                  </div>
                ))}
              </dl>
            </section>
          )}

          {hasRewards && (
            <section>
              <header>
                <h4>Reward points</h4>
                {rewards?.closing !== undefined && <strong>{points(rewards.closing)}</strong>}
              </header>
              <dl>
                {rewardRows.map(([label, value, sign]) => (
                  <div key={label}>
                    <dt>{label}</dt>
                    <dd className={sign === '+' ? styles.inflow : undefined}>
                      {sign}
                      {points(value)}
                    </dd>
                  </div>
                ))}
                {section.rewardPointsOnRows !== undefined && (
                  <div>
                    <dt>
                      Printed on rows
                      <small>added up</small>
                    </dt>
                    <dd>{points(section.rewardPointsOnRows)}</dd>
                  </div>
                )}
              </dl>
            </section>
          )}
        </div>
      )}

      {plans.length > 0 && (
        <section className={styles.emis}>
          <header>
            <h4>EMIs</h4>
            <span>
              <strong>{money(plans.reduce((sum, p) => sum + (p.outstanding ?? 0), 0))}</strong> still to pay
            </span>
          </header>
          <table>
            <thead>
              <tr>
                <th>Plan</th>
                <th>Paid so far</th>
                <th className={styles.num}>Each month</th>
                <th className={styles.num}>Interest</th>
                <th className={styles.num}>Left to pay</th>
              </tr>
            </thead>
            <tbody>
              {plans.map((p, i) => {
                const paidOff = p.outstanding === 0 || p.remainingMonths === 0;
                const paidMonths =
                  p.tenureMonths !== undefined && p.remainingMonths !== undefined
                    ? p.tenureMonths - p.remainingMonths
                    : null;
                return (
                  <tr key={`${p.description}-${i}`} className={paidOff ? styles.emiDone : undefined}>
                    <td>{p.description || `EMI ${i + 1}`}</td>
                    <td>
                      {paidOff ? (
                        'Paid off'
                      ) : paidMonths !== null && p.tenureMonths ? (
                        <span className={styles.emiProgress}>
                          <span className={styles.meter} aria-hidden>
                            <span style={{ width: `${Math.max((paidMonths / p.tenureMonths) * 100, 2)}%` }} />
                          </span>
                          {paidMonths} of {p.tenureMonths} months
                        </span>
                      ) : p.remainingMonths !== undefined ? (
                        `${p.remainingMonths} months left`
                      ) : (
                        '—'
                      )}
                    </td>
                    <td className={styles.num}>{p.monthlyAmount !== undefined ? money(p.monthlyAmount) : '—'}</td>
                    <td className={styles.num}>{p.interestRate !== undefined ? `${p.interestRate}%` : '—'}</td>
                    <td className={styles.num}>{p.outstanding !== undefined ? money(p.outstanding) : '—'}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          <p>Instalments sit under "Covered by EMI", not Missing: the purchase is already in your budget.</p>
        </section>
      )}
    </div>
  );
}
