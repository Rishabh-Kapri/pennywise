import { Link } from 'react-router-dom';
import {
  ArrowRight,
  CalendarBlank,
  ChatCircleText,
  ChartPie,
  EnvelopeSimple,
  GithubLogo,
  LockKey,
  MagicWand,
  PencilSimpleLine,
  Receipt,
  ShieldCheck,
  Sparkle as Sparkles,
  TrendUp,
  Wallet,
} from '@phosphor-icons/react';
import styles from './Homepage.module.css';

const budgetGroups = [
  {
    name: 'Home Base',
    assigned: '₹42,600',
    activity: '-₹21,380',
    available: '₹21,220',
    categories: [
      { name: 'Utilities', assigned: '₹8,200', activity: '-₹3,460', available: '₹4,740' },
      { name: 'Household', assigned: '₹6,400', activity: '-₹2,920', available: '₹3,480' },
    ],
  },
  {
    name: 'Daily Spend',
    assigned: '₹31,750',
    activity: '-₹18,240',
    available: '₹13,510',
    categories: [
      { name: 'Groceries', assigned: '₹16,000', activity: '-₹9,640', available: '₹6,360' },
      { name: 'Commute', assigned: '₹4,800', activity: '-₹2,150', available: '₹2,650' },
    ],
  },
  {
    name: 'Future Plans',
    assigned: '₹26,000',
    activity: '-₹4,250',
    available: '₹21,750',
    categories: [
      { name: 'Trip fund', assigned: '₹12,000', activity: '-₹1,800', available: '₹10,200' },
      { name: 'Emergency buffer', assigned: '₹14,000', activity: '-₹2,450', available: '₹11,550' },
    ],
  },
];

const agentMessages = [
  { role: 'user', text: 'how is this month looking?' },
  {
    role: 'agent',
    text: 'You have room in daily spend, but utilities is trending higher than usual.',
  },
  { role: 'user', text: 'what should i check next?' },
  {
    role: 'agent',
    text: 'I can summarize category drift, find unusual transactions, or help move available money.',
  },
];

const transactionRows = [
  {
    account: 'Nova Credit',
    date: '27 Jun 2026',
    payee: 'City Power',
    category: 'Utilities',
    amount: '-₹1,860',
  },
  {
    account: 'Everyday Bank',
    date: '27 Jun 2026',
    payee: 'Fresh Basket',
    category: 'Groceries',
    amount: '-₹2,180',
  },
  {
    account: 'Travel Wallet',
    date: '25 Jun 2026',
    payee: 'Metro Tap',
    category: 'Commute',
    amount: '-₹620',
  },
  {
    account: 'Everyday Bank',
    date: '24 Jun 2026',
    payee: 'Design Tools',
    category: 'Software',
    amount: '-₹899',
  },
];

const incomingTransaction = {
  account: 'Inbox Import',
  date: 'Just now',
  payee: 'Streamline Fiber',
  category: 'Internet',
  amount: '-₹1,240',
};

const predictionStages = [
  {
    icon: <EnvelopeSimple size={20} weight="bold" />,
    eyebrow: 'Gmail ingestion',
    title: 'A receipt lands in the watched inbox.',
    description:
      'Cipher pulls the message metadata, extracts the merchant, amount, and card context, then sends only the useful transaction signal forward.',
    screen: (
      <div className={`${styles.flowScreen} ${styles.emailScreen}`}>
        <div className={styles.flowScreenHeader}>
          <span>Inbox</span>
          <strong>New receipt</strong>
        </div>
        <div className={styles.emailCard}>
          <span>From: receipts@streamline.example</span>
          <strong>Streamline Fiber payment received</strong>
          <p>Paid with Nova Credit ending 4281</p>
          <div className={styles.receiptTotal}>
            <span>Total</span>
            <strong>₹1,240</strong>
          </div>
        </div>
        <div className={styles.emailStatus}>
          <i />
          Parsed merchant, amount, and account hint
        </div>
      </div>
    ),
  },
  {
    icon: <MagicWand size={20} weight="bold" />,
    eyebrow: 'Prediction',
    title: 'The classifier proposes account, payee, and category.',
    description:
      'Cipher scores signals from recurring payments, recent corrections, and learned category patterns before creating anything permanent.',
    screen: (
      <div className={`${styles.flowScreen} ${styles.predictionScreen}`}>
        <div className={styles.flowScreenHeader}>
          <span>Prediction run</span>
          <strong>Ready for creation</strong>
        </div>
        <div className={styles.predictionGrid}>
          <span>
            Account <strong>Nova Credit</strong>
          </span>
          <span>
            Payee <strong>Streamline Fiber</strong>
          </span>
          <span>
            Category <strong>Internet</strong>
          </span>
        </div>
        <div className={styles.confidenceList}>
          <div className={styles.confidenceRow}>
            <span>Recurring payee match</span>
            <i />
            <strong>96%</strong>
          </div>
          <div className={styles.confidenceRow}>
            <span>Category history</span>
            <i />
            <strong>91%</strong>
          </div>
          <div className={styles.confidenceRow}>
            <span>Account signal</span>
            <i />
            <strong>88%</strong>
          </div>
        </div>
      </div>
    ),
  },
  {
    icon: <Receipt size={20} weight="bold" />,
    eyebrow: 'Transaction creation',
    title: 'The prediction becomes a reviewable transaction.',
    description:
      'The transaction appears in the ledger with the predicted category and source context, ready for approval or correction.',
    screen: (
      <div className={`${styles.flowScreen} ${styles.flowTransactions}`}>
        <div className={styles.flowScreenHeader}>
          <span>Transactions</span>
          <strong>June 2026</strong>
        </div>
        <div className={styles.flowTransactionColumns}>
          <span>Account</span>
          <span>Payee</span>
          <span>Category</span>
          <span>Amount</span>
        </div>
        <div className={styles.flowTransactionRow}>
          <span>Everyday Bank</span>
          <strong>Fresh Basket</strong>
          <em>Groceries</em>
          <b>-₹2,180</b>
        </div>
        <div className={`${styles.flowTransactionRow} ${styles.flowCreatedRow}`}>
          <span>{incomingTransaction.account}</span>
          <strong>{incomingTransaction.payee}</strong>
          <em>{incomingTransaction.category}</em>
          <b>{incomingTransaction.amount}</b>
        </div>
        <div className={styles.flowTransactionRow}>
          <span>Travel Wallet</span>
          <strong>Metro Tap</strong>
          <em>Commute</em>
          <b>-₹620</b>
        </div>
      </div>
    ),
  },
  {
    icon: <PencilSimpleLine size={20} weight="bold" />,
    eyebrow: 'Correction',
    title: 'If the prediction is wrong, the fix trains the next one.',
    description:
      'Changing a predicted field records exactly what was corrected, so the same payee or pattern can improve without extra manual setup.',
    screen: (
      <div className={`${styles.flowScreen} ${styles.correctionScreen}`}>
        <div className={styles.flowScreenHeader}>
          <span>Correction</span>
          <strong>User reviewed</strong>
        </div>
        <div className={styles.correctionCard}>
          <span>Streamline Fiber</span>
          <strong>-₹1,240</strong>
          <div className={styles.correctionField}>
            <small>Predicted category</small>
            <b>Utilities</b>
          </div>
          <div className={styles.correctionField}>
            <small>User correction</small>
            <b>Internet</b>
          </div>
          <button type="button">Save correction</button>
        </div>
      </div>
    ),
  },
  {
    icon: <TrendUp size={20} weight="bold" />,
    eyebrow: 'Learning loop',
    title: 'Cipher remembers the correction for future receipts.',
    description:
      'The next matching email can be categorized with higher confidence, and Cipher has better context when explaining monthly spending.',
    screen: (
      <div className={`${styles.flowScreen} ${styles.learningScreen}`}>
        <div className={styles.flowScreenHeader}>
          <span>Learned pattern</span>
          <strong>Active</strong>
        </div>
        <div className={styles.learningCard}>
          <span>Payee pattern</span>
          <strong>Streamline Fiber → Internet</strong>
          <p>Applied to recurring broadband receipts from Nova Credit.</p>
        </div>
        <div className={styles.learningMetric}>
          <span>Next match confidence</span>
          <strong>97%</strong>
        </div>
      </div>
    ),
  },
];

const highlights = [
  {
    icon: <ChartPie size={20} />,
    title: 'Know where money goes',
    description: 'Track budgets, accounts, and transactions from one focused workspace.',
  },
  {
    icon: <Wallet size={20} />,
    title: 'Move faster each month',
    description: 'Plan categories, review spending, and keep monthly choices visible.',
  },
  {
    icon: <ShieldCheck size={20} />,
    title: 'Keep context protected',
    description: 'Budget access, auth, and account flows stay scoped around your data.',
  },
];

export default function Homepage() {
  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <Link to="/" className={styles.brand} aria-label="Pennywise home">
          <span className={styles.logo}>P</span>
          <span>Pennywise</span>
        </Link>

        <nav className={styles.actions} aria-label="Account actions">
          <a
            href="https://github.com/Rishabh-Kapri/pennywise"
            className={styles.githubButton}
            target="_blank"
            rel="noreferrer"
            aria-label="Pennywise on GitHub">
            <GithubLogo size={20} weight="bold" />
          </a>
          <a href="#terms" className={styles.loginButton}>
            Terms
          </a>
          <Link to="/login" className={styles.loginButton}>
            Login
          </Link>
          <Link to="/signup" className={styles.signupButton}>
            Sign up
          </Link>
        </nav>
      </header>

      <section className={styles.hero}>
        <div className={styles.heroTexture} aria-hidden="true" />

        <div className={styles.heroVisual} aria-hidden="true">
          <div className={styles.appFrame}>
            <div className={styles.windowBar}>
              <span />
              <span />
              <span />
              <strong>Pennywise / June Budget</strong>
            </div>
            <div className={styles.appWorkspace}>
              <section className={styles.budgetShot}>
                <div className={styles.monthPill}>
                  <CalendarBlank size={14} weight="bold" />
                  <span>June, 2026</span>
                </div>
                <div className={styles.shotHeader}>
                  <div>
                    <span>Monthly plan</span>
                    <strong>Budget</strong>
                  </div>
                  <em>48 categories</em>
                </div>
                <div className={styles.summaryStrip}>
                  <span>
                    Assigned <strong>₹1,00,350</strong>
                  </span>
                  <span>
                    Activity <strong>-₹43,870</strong>
                  </span>
                  <span>
                    Available <strong>₹56,480</strong>
                  </span>
                </div>
                <div className={styles.categoryTable}>
                  <div className={styles.tableHeader}>
                    <span />
                    <span>Assigned</span>
                    <span>Activity</span>
                    <span>Available</span>
                  </div>
                  {budgetGroups.map((group) => (
                    <div className={styles.groupCard} key={group.name}>
                      <div className={styles.groupRow}>
                        <strong>{group.name}</strong>
                        <span>{group.assigned}</span>
                        <span>{group.activity}</span>
                        <span>{group.available}</span>
                      </div>
                      {group.categories.map((category) => (
                        <div className={styles.categoryRow} key={category.name}>
                          <span>{category.name}</span>
                          <small>{category.assigned}</small>
                          <small>{category.activity}</small>
                          <strong>{category.available}</strong>
                        </div>
                      ))}
                    </div>
                  ))}
                </div>
              </section>
              <aside className={styles.budgetAside}>
                <h2>June Budget</h2>
                <div>
                  <span>Total Available</span>
                  <strong>₹56,480</strong>
                </div>
                <small>Total assigned ₹1,00,350</small>
                <small>Total activity -₹43,870</small>
              </aside>
            </div>
          </div>

          <div className={styles.transactionsCard}>
            <div className={styles.transactionsToolbar}>
              <div>
                <span>All accounts</span>
                <strong>₹8,42,190</strong>
              </div>
              <em>Search transactions</em>
            </div>
            <div className={styles.transactionsContent}>
              <section className={styles.transactionsTable}>
                <div className={styles.transactionColumns}>
                  <span>Account</span>
                  <span>Date</span>
                  <span>Payee</span>
                  <span>Category</span>
                  <span>Amount</span>
                </div>
                <div className={styles.transactionMonth}>
                  <span>June 2026</span>
                  <small>25 transactions</small>
                </div>
                <div
                  className={`${styles.transactionPreviewRow} ${styles.selectedTransaction} ${styles.heroIncomingTransaction}`}>
                  <span>{incomingTransaction.account}</span>
                  <span>{incomingTransaction.date}</span>
                  <strong>{incomingTransaction.payee}</strong>
                  <em>{incomingTransaction.category}</em>
                  <b>{incomingTransaction.amount}</b>
                </div>
                {transactionRows.map((row) => (
                  <div
                    className={styles.transactionPreviewRow}
                    key={`${row.payee}-${row.amount}`}>
                    <span>{row.account}</span>
                    <span>{row.date}</span>
                    <strong>{row.payee}</strong>
                    <em>{row.category}</em>
                    <b>{row.amount}</b>
                  </div>
                ))}
              </section>
              <aside className={styles.transactionDetail}>
                <span>Transaction</span>
                <strong>-₹1,240</strong>
                <em>Internet</em>
                <small>Streamline Fiber</small>
                <small>Nova Credit</small>
                <i>Approved</i>
                <p>Created from an email receipt after prediction review.</p>
              </aside>
            </div>
          </div>

          <div className={styles.agentDemo}>
            <div className={styles.agentHeader}>
              <span>
                <ChatCircleText size={18} weight="bold" />
              </span>
              <strong>Cipher</strong>
              <i />
            </div>
            <div className={styles.agentBody}>
              {agentMessages.map((message, index) => (
                <div
                  className={
                    message.role === 'user'
                      ? `${styles.agentBubble} ${styles.userBubble}`
                      : `${styles.agentBubble} ${styles.pennyBubble}`
                  }
                  key={message.text}
                  style={{ animationDelay: `${index * 0.72}s` }}>
                  {message.role === 'agent' && index === 1 && (
                    <span className={styles.contextChip}>
                      <Sparkles size={13} weight="fill" />
                      Loaded budget context
                    </span>
                  )}
                  {message.text}
                </div>
              ))}
              <div className={styles.typingBubble}>
                <span />
                <span />
                <span />
              </div>
            </div>
            <div className={styles.agentPrompts}>
              {['Summarize spending', 'Find unusual transactions', 'Plan next move'].map((prompt) => (
                <span key={prompt}>{prompt}</span>
              ))}
            </div>
            <div className={styles.agentComposer}>How can I help you today?</div>
          </div>
        </div>

        <div className={styles.heroContent}>
          <p className={styles.eyebrow}>
            <Sparkles size={15} weight="fill" />
            Prediction-aware personal budgeting
          </p>
          <h1>Secure, flexible, and transparent money tracking</h1>
          <p className={styles.subcopy}>
            Pennywise brings monthly planning, transaction review, and prediction-assisted
            categorization into one calm finance workspace.
          </p>
          <div className={styles.ctaRow}>
            <Link to="/signup" className={styles.primaryCta}>
              Start budgeting <ArrowRight size={18} weight="bold" />
            </Link>
            <Link to="/login" className={styles.secondaryCta}>
              <LockKey size={18} weight="bold" />
              I already have an account
            </Link>
          </div>
        </div>
      </section>

      <section className={styles.predictionStory} aria-labelledby="prediction-flow-title">
        <div className={styles.storyIntro}>
          <span>Prediction flow</span>
          <h2 id="prediction-flow-title">From receipt email to a cleaner budget.</h2>
        </div>

        <div className={styles.predictionStack}>
          {predictionStages.map((stage, index) => (
            <article className={styles.predictionPanel} key={stage.title}>
              <div className={styles.predictionCopy}>
                <span className={styles.stepNumber}>0{index + 1}</span>
                <p className={styles.stepEyebrow}>
                  {stage.icon}
                  {stage.eyebrow}
                </p>
                <h3>{stage.title}</h3>
                <p>{stage.description}</p>
              </div>
              <div className={styles.predictionVisual}>{stage.screen}</div>
            </article>
          ))}
        </div>
      </section>

      <section className={styles.highlights} aria-label="Pennywise highlights">
        {highlights.map((highlight) => (
          <article className={styles.highlightCard} key={highlight.title}>
            <div className={styles.highlightIcon}>{highlight.icon}</div>
            <h2>{highlight.title}</h2>
            <p>{highlight.description}</p>
          </article>
        ))}
      </section>

      <section id="terms" className={styles.closingSection} aria-label="Get started with Pennywise">
        <div className={styles.closingCopy}>
          <span className={styles.legalKicker}>Built for review</span>
          <h2>Close the loop on every transaction.</h2>
          <p>
            Pennywise keeps the automation visible: Cipher can predict, you can correct,
            and the next receipt gets easier to place.
          </p>
          <div className={styles.closingActions}>
            <Link to="/signup" className={styles.primaryCta}>
              Start budgeting <ArrowRight size={18} weight="bold" />
            </Link>
            <a
              href="https://github.com/Rishabh-Kapri/pennywise"
              className={styles.secondaryCta}
              target="_blank"
              rel="noreferrer">
              <GithubLogo size={18} weight="bold" />
              View GitHub
            </a>
          </div>
        </div>

        <aside className={styles.closingPanel}>
          <div className={styles.closingPanelHeader}>
            <span>
              <ShieldCheck size={18} weight="bold" />
            </span>
            <strong>Workspace ready</strong>
          </div>
          <div className={styles.closingStatus}>
            <span>Cipher loop</span>
            <strong>Gmail to prediction to ledger</strong>
            <small>Corrections feed the next import.</small>
          </div>
          <div className={styles.trustRows}>
            <div>
              <span>Budget scoped</span>
              <strong>Imports stay tied to the selected budget.</strong>
            </div>
            <div>
              <span>Review first</span>
              <strong>Predictions remain editable before you rely on them.</strong>
            </div>
            <div>
              <span>Open source</span>
              <strong>Inspect the app, API, ingestion, MLP, and Cipher.</strong>
            </div>
          </div>
          <div className={styles.legalLinks}>
            <Link to="/terms">Terms</Link>
            <Link id="privacy" to="/privacy">
              Privacy
            </Link>
          </div>
        </aside>
      </section>
    </main>
  );
}
