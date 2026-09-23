import { useTheme } from '@/context/ThemeContext';
import { ThemeToggle } from '@/components/theme/ThemeToggle';
import { Link } from 'react-router-dom';
import {
  ArrowDownLeft,
  ArrowRight,
  ArrowUpRight,
  CalendarBlank,
  ChartPie,
  Check,
  CheckCircle,
  EnvelopeSimple,
  GithubLogo,
  Plant,
  Receipt,
  ShieldCheck,
  Sparkle,
  Wallet,
} from '@phosphor-icons/react';
import styles from './Homepage.module.css';

const categories = [
  { name: 'Home & essentials', spent: 21380, assigned: 42600, color: 'periwinkle' },
  { name: 'Everyday spending', spent: 18240, assigned: 31750, color: 'blue' },
  { name: 'Future plans', spent: 4250, assigned: 26000, color: 'peach' },
];
const inr = (value: number) => `₹${value.toLocaleString('en-IN')}`;
const steps = [
  {
    icon: EnvelopeSimple,
    title: 'A receipt arrives.',
    description: 'Bring payment emails from your connected Gmail inbox into your budget.',
  },
  {
    icon: Sparkle,
    title: 'Cipher sorts the details.',
    description: 'Get a suggested payee, account, and category, without entering every field.',
  },
  {
    icon: CheckCircle,
    title: 'You make the final call.',
    description: 'Review your transactions and correct anything. Cipher learns from your changes.',
  },
];

function BudgetPreview() {
  return (
    <figure className={styles.preview} aria-label="Example monthly budget with ₹56,480 available">
      <div className={styles.previewBar}>
        <span className={styles.previewBrand}>
          <span className={styles.smallLogo}>P</span> My workspace
        </span>
        <span className={styles.sampleBadge}>Sample budget</span>
      </div>
      <div className={styles.previewBody}>
        <div className={styles.budgetHeading}>
          <div>
            <p>YOUR MONEY, AT A GLANCE</p>
            <h2>Monthly overview</h2>
          </div>
          <span className={styles.month}>
            <CalendarBlank size={14} /> June 2026
          </span>
        </div>
        <div className={styles.balance}>
          <span>Available to spend</span>
          <strong>
            ₹56,480<span>.00</span>
          </strong>
          <span className={styles.balanceNote}>
            <CheckCircle size={14} weight="fill" /> Every rupee has a place.
          </span>
        </div>
        <div className={styles.metrics}>
          <div>
            <span>
              <ArrowDownLeft size={15} /> Assigned
            </span>
            <strong>₹1,00,350</strong>
          </div>
          <div>
            <span>
              <ArrowUpRight size={15} /> Spent this month
            </span>
            <strong>₹43,870</strong>
          </div>
        </div>
        <div className={styles.categoryHeading}>
          <strong>Your categories</strong>
          <span>Spent / assigned</span>
        </div>
        <div className={styles.categories}>
          {categories.map((category) => (
            <div className={styles.category} key={category.name}>
              <div>
                <span>{category.name}</span>
                <span>
                  {inr(category.spent)} <em>/ {inr(category.assigned)}</em>
                </span>
              </div>
              <div className={styles.track}>
                <span
                  className={styles[category.color]}
                  style={{ width: `${(category.spent / category.assigned) * 100}%` }}
                />
              </div>
            </div>
          ))}
        </div>
        <div className={styles.cipherNote}>
          <span className={styles.cipherIcon}>
            <Sparkle size={19} weight="fill" />
          </span>
          <div>
            <strong>A little clarity, courtesy of Cipher</strong>
            <p>You have ₹21,750 left for your future plans.</p>
          </div>
        </div>
      </div>
      <figcaption className={styles.previewCaption}>
        <span className={styles.statusDot} /> A clearer picture. A calmer month.
      </figcaption>
    </figure>
  );
}

export default function Homepage() {
  const { theme } = useTheme();
  return (
    <main className={styles.page} data-home-theme={theme} tabIndex={0} aria-label="Pennywise home">
      <a href="#main-content" className={styles.skipLink}>
        Skip to content
      </a>
      <header className={styles.header}>
        <Link to="/" className={styles.brand} aria-label="Pennywise home">
          <span className={styles.logo}>P</span>Pennywise<span className={styles.brandDot}>.</span>
        </Link>
        <nav className={styles.navigation} aria-label="Main navigation">
          <a href="#features">Features</a>
          <a href="#how-it-works">How it works</a>
          <a href="https://github.com/Rishabh-Kapri/pennywise" target="_blank" rel="noreferrer">
            Open source <ArrowUpRight size={13} />
          </a>
        </nav>
        <div className={styles.accountActions}>
          <ThemeToggle />
          <Link to="/login" className={styles.login}>
            Log in
          </Link>
          <Link to="/signup" className={styles.headerCta}>
            Get started <ArrowUpRight size={15} />
          </Link>
        </div>
      </header>

      <section id="main-content" className={styles.hero} aria-labelledby="hero-title">
        <img
          className={styles.heroPhoto}
          src="/images/budget-desk-envelopes.webp"
          alt=""
          fetchPriority="high"
          width={1536}
          height={1024}
        />
        <div className={styles.heroCopy}>
          <p className={styles.eyebrow}>
            <span className={styles.statusDot} /> A little wiser with every rupee
          </p>
          <h1 id="hero-title">
            Your money.
            <br />
            Less noise.
            <br />
            <span>More clarity.</span>
          </h1>
          <p className={styles.heroDescription}>
            Make room for what matters. Bring your budgets, spending, and everyday decisions together in one thoughtful
            workspace.
          </p>
          <div className={styles.heroActions}>
            <Link to="/signup" className={styles.primaryCta}>
              Start budgeting <ArrowRight size={18} />
            </Link>
            <a href="#how-it-works" className={styles.textCta}>
              See how it works <ArrowDownLeft size={17} />
            </a>
          </div>
          <div className={styles.heroDetails}>
            <span>
              <Check size={14} /> Open source
            </span>
            <span>
              <Check size={14} /> AI-assisted categorization
            </span>
          </div>
        </div>
      </section>
      <section className={styles.productSection} aria-labelledby="overview-title">
        <div className={styles.productIntro}>
          <p className={styles.eyebrow}>A PLACE FOR THE BIG PICTURE</p>
          <h2 id="overview-title">
            Less guesswork.
            <br />
            More peace of mind.
          </h2>
          <p>
            From your morning coffee to your next big adventure, see how today’s spending fits into tomorrow’s plans.
          </p>
        </div>
        <div className={styles.heroVisual}>
          <BudgetPreview />
          <div className={styles.visualFootnote}>
            <ShieldCheck size={15} /> Your budget. Your decisions. Always.
          </div>
        </div>
      </section>

      <section id="features" className={styles.features} aria-labelledby="features-title">
        <div className={styles.sectionHeading}>
          <p className={styles.eyebrow}>LESS BUSYWORK. MORE BIG PICTURE.</p>
          <h2 id="features-title">Everything in its right place.</h2>
          <p>A simpler rhythm for your everyday finances.</p>
        </div>
        <div className={styles.featureGrid}>
          <article className={styles.featureCard}>
            <span className={styles.featureIcon}>
              <Wallet size={24} />
            </span>
            <h3>A plan for every rupee.</h3>
            <p>
              Give your money a purpose with monthly categories. See what’s spent, what’s left, and where you can
              adjust.
            </p>
            <div className={styles.featureIllustration}>
              <span>
                <Plant size={18} /> Next adventure
              </span>
              <strong>
                ₹10,200 <small>left to grow</small>
              </strong>
              <div className={styles.savingsTrack}>
                <span />
              </div>
            </div>
          </article>
          <article className={styles.featureCard}>
            <span className={styles.featureIcon}>
              <Receipt size={24} />
            </span>
            <h3>All the little details, together.</h3>
            <p>Keep accounts and transactions in one view, so a quick check-in gives you the full picture.</p>
            <div className={styles.miniTransaction}>
              <span className={styles.merchantIcon}>F</span>
              <span>
                <strong>Fresh Basket</strong>
                <small>Groceries · Everyday Bank</small>
              </span>
              <b>−₹2,180</b>
            </div>
          </article>
          <article className={styles.featureCard}>
            <span className={styles.featureIcon}>
              <ChartPie size={24} />
            </span>
            <h3>A smarter second look.</h3>
            <p>
              Let Cipher help categorize receipts and explore your spending. Stay in control with predictions you can
              review and correct.
            </p>
            <div className={styles.aiIllustration}>
              <Sparkle size={17} weight="fill" />
              <span>
                Less sorting.
                <br />
                <strong>More understanding.</strong>
              </span>
            </div>
          </article>
        </div>
      </section>

      <section id="how-it-works" className={styles.workflow} aria-labelledby="workflow-title">
        <div className={styles.workflowIntro}>
          <p className={styles.eyebrow}>MEET CIPHER</p>
          <h2 id="workflow-title">
            From inbox
            <br />
            to insight.
          </h2>
          <p>
            A helping hand behind the scenes.
            <br />
            You’re still in the driver’s seat.
          </p>
        </div>
        <ol className={styles.steps}>
          {steps.map(({ icon: Icon, title, description }, index) => (
            <li key={title}>
              <span className={styles.stepIcon}>
                <Icon size={21} />
              </span>
              <div>
                <span className={styles.stepNumber}>0{index + 1}</span>
                <h3>{title}</h3>
                <p>{description}</p>
              </div>
            </li>
          ))}
        </ol>
      </section>

      <section className={styles.closing} aria-labelledby="closing-title">
        <span className={styles.closingIcon}>
          <Plant size={30} />
        </span>
        <p className={styles.eyebrow}>SMALL STEPS. A CLEARER TOMORROW.</p>
        <h2 id="closing-title">
          Good with money
          <br />
          starts with a little clarity.
        </h2>
        <Link to="/signup" className={styles.primaryCta}>
          Make room for what matters <ArrowRight size={18} />
        </Link>
      </section>
      <footer className={styles.footer}>
        <Link to="/" className={styles.brand}>
          <span className={styles.logo}>P</span>Pennywise<span className={styles.brandDot}>.</span>
        </Link>
        <p>A little more intention. Every day.</p>
        <nav aria-label="Footer navigation">
          <Link to="/terms">Terms</Link>
          <Link to="/privacy">Privacy</Link>
          <a
            href="https://github.com/Rishabh-Kapri/pennywise"
            target="_blank"
            rel="noreferrer"
            aria-label="Pennywise on GitHub"
          >
            <GithubLogo size={20} />
          </a>
        </nav>
      </footer>
    </main>
  );
}
