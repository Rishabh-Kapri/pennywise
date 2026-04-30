import { Link, useLocation } from 'react-router-dom';
import styles from './LegalPage.module.css';

const content = {
  terms: {
    title: 'Terms and Conditions',
    intro:
      'These terms explain the expected use of Pennywise as a personal budgeting workspace.',
    sections: [
      {
        title: 'Personal finance planning',
        body: 'Pennywise helps organize budgets, accounts, transactions, and predictions. It does not replace professional financial advice or your own review of financial decisions.',
      },
      {
        title: 'Account responsibility',
        body: 'You are responsible for keeping your login secure, reviewing imported information, and ensuring data you connect or enter is accurate.',
      },
      {
        title: 'Service changes',
        body: 'Features may change as the product evolves. We aim to keep budgeting workflows reliable while improving the service over time.',
      },
    ],
  },
  privacy: {
    title: 'Privacy Policy',
    intro:
      'This policy summarizes how Pennywise should handle information used to power your budgeting experience.',
    sections: [
      {
        title: 'Information used by the app',
        body: 'Pennywise may use account, budget, transaction, category, and authentication information to provide budgeting, categorization, and reporting features.',
      },
      {
        title: 'Limited purpose',
        body: 'Your data should be used only to operate, secure, debug, and improve the product experience you requested.',
      },
      {
        title: 'Security expectations',
        body: 'Access to sensitive financial context should be protected with appropriate authentication, authorization, and service-level controls.',
      },
    ],
  },
};

export default function LegalPage() {
  const { pathname } = useLocation();
  const page = pathname.includes('privacy') ? content.privacy : content.terms;

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <Link to="/" className={styles.brand}>
          Pennywise
        </Link>
        <Link to="/login" className={styles.loginLink}>
          Login
        </Link>
      </header>

      <section className={styles.content}>
        <p className={styles.kicker}>Legal</p>
        <h1>{page.title}</h1>
        <p className={styles.intro}>{page.intro}</p>

        <div className={styles.sectionList}>
          {page.sections.map((section) => (
            <article className={styles.sectionCard} key={section.title}>
              <h2>{section.title}</h2>
              <p>{section.body}</p>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}
