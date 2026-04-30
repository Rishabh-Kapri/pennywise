import { Link } from 'react-router-dom';
import { ArrowRight, ChartPie, ShieldCheck, Sparkles } from 'lucide-react';
import styles from './Homepage.module.css';

const highlights = [
  {
    icon: <ChartPie size={20} />,
    title: 'Know where money goes',
    description: 'Track budgets, accounts, and transactions from one calm dashboard.',
  },
  {
    icon: <Sparkles size={20} />,
    title: 'Categorize faster',
    description: 'Use prediction-assisted categorization to spend less time cleaning data.',
  },
  {
    icon: <ShieldCheck size={20} />,
    title: 'Built for your budget',
    description: 'Keep personal finance workflows focused around monthly planning.',
  },
];

export default function Homepage() {
  return (
    <main className={styles.page}>
      <div className={styles.backgroundPattern} aria-hidden="true" />

      <header className={styles.header}>
        <Link to="/" className={styles.brand} aria-label="Pennywise home">
          <span className={styles.logo}>P</span>
          <span>Pennywise</span>
        </Link>

        <nav className={styles.actions} aria-label="Account actions">
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
        <div className={styles.heroContent}>
          <p className={styles.eyebrow}>Personal budgeting without the spreadsheet fog</p>
          <h1>Make every rupee easier to place, track, and trust.</h1>
          <p className={styles.subcopy}>
            Pennywise helps you plan monthly budgets, monitor accounts, and turn
            transaction noise into clear financial decisions.
          </p>
          <div className={styles.ctaRow}>
            <Link to="/signup" className={styles.primaryCta}>
              Start budgeting <ArrowRight size={18} />
            </Link>
            <Link to="/login" className={styles.secondaryCta}>
              I already have an account
            </Link>
          </div>
        </div>

        <div className={styles.previewCard} aria-label="Budget overview preview">
          <div className={styles.previewTopline}>
            <span>April budget</span>
            <strong>On track</strong>
          </div>
          <div className={styles.amountBlock}>
            <span>Available to assign</span>
            <strong>₹24,800</strong>
          </div>
          <div className={styles.barGroup}>
            <div className={styles.barLabel}>
              <span>Essentials</span>
              <span>72%</span>
            </div>
            <div className={styles.track}>
              <span className={styles.barEssentials} />
            </div>
            <div className={styles.barLabel}>
              <span>Savings</span>
              <span>48%</span>
            </div>
            <div className={styles.track}>
              <span className={styles.barSavings} />
            </div>
            <div className={styles.barLabel}>
              <span>Dining out</span>
              <span>31%</span>
            </div>
            <div className={styles.track}>
              <span className={styles.barDining} />
            </div>
          </div>
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

      <section className={styles.legalPreview} aria-label="Legal information">
        <article id="terms" className={styles.legalCard}>
          <span className={styles.legalKicker}>Terms and Conditions</span>
          <h2>Use Pennywise as your planning workspace.</h2>
          <p>
            Pennywise is intended for personal budgeting and finance organization.
            You are responsible for reviewing imported transactions, keeping account
            access secure, and confirming any decisions before acting on them.
          </p>
          <Link to="/terms" className={styles.legalLink}>
            Read terms
          </Link>
        </article>

        <article id="privacy" className={styles.legalCard}>
          <span className={styles.legalKicker}>Privacy</span>
          <h2>Your financial context should stay protected.</h2>
          <p>
            We use your account data to power budgeting, categorization, and app
            functionality. Access should be limited to what is needed to provide the
            service and improve your experience.
          </p>
          <Link to="/privacy" className={styles.legalLink}>
            Read privacy policy
          </Link>
        </article>
      </section>
    </main>
  );
}
