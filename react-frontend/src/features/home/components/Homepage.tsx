import { Link } from 'react-router-dom';
import { ArrowRight, ChartPie, ShieldCheck, Sparkle as Sparkles } from '@phosphor-icons/react';
import styles from './Homepage.module.css';

/* ── Inline vector glyphs used as floating decorations ── */

const GlyphCircle = ({ size = 32 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
    <circle cx="16" cy="16" r="14" stroke="currentColor" strokeWidth="1.5" />
    <circle cx="16" cy="16" r="6" stroke="currentColor" strokeWidth="1" opacity="0.5" />
  </svg>
);

const GlyphCross = ({ size = 28 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 28 28" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M14 4v20M4 14h20" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
  </svg>
);

const GlyphDiamond = ({ size = 30 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 30 30" fill="none" xmlns="http://www.w3.org/2000/svg">
    <rect x="15" y="2" width="18" height="18" rx="3" transform="rotate(45 15 2)" stroke="currentColor" strokeWidth="1.5" />
  </svg>
);

const GlyphTriangle = ({ size = 26 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 26 26" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M13 4L23 22H3L13 4Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
  </svg>
);

const GlyphHex = ({ size = 34 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 34 34" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M17 3L29 10v14l-12 7L5 24V10L17 3Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
  </svg>
);

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
      {/* Dot-grid background */}
      <div className={styles.bgDots} aria-hidden="true" />

      {/* Floating vector glyphs */}
      <div className={`${styles.glyphFloat} ${styles.glyph1}`} aria-hidden="true">
        <GlyphCircle size={40} />
      </div>
      <div className={`${styles.glyphFloat} ${styles.glyph2}`} aria-hidden="true">
        <GlyphCross size={24} />
      </div>
      <div className={`${styles.glyphFloat} ${styles.glyph3}`} aria-hidden="true">
        <GlyphDiamond size={36} />
      </div>
      <div className={`${styles.glyphFloat} ${styles.glyph4}`} aria-hidden="true">
        <GlyphTriangle size={28} />
      </div>
      <div className={`${styles.glyphFloat} ${styles.glyph5}`} aria-hidden="true">
        <GlyphHex size={38} />
      </div>

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
          <p className={styles.eyebrow}>
            <Sparkles size={14} className={styles.eyebrowIcon} />
            Personal budgeting without the spreadsheet fog
          </p>
          <h1>Make every rupee easier to <span className={styles.heroAccent}>place, track,</span> and trust.</h1>
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
