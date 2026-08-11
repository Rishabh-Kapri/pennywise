import {
  ArrowsClockwise,
  GearSix,
  ListChecks,
  Money as Banknote,
  Pulse,
  Robot,
  Tag,
} from '@phosphor-icons/react';
import { lazy, Suspense, useEffect, useState, type ReactNode } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { selectActivePipelineRunCount } from '@/features/pipeline/store';
import { apiClient } from '@/utils';
import { GeneralSettings } from './GeneralSettings';
import { AISettings } from './AISettings';
import { TagSettings } from './TagSettings';

// heavier, data-driven sections: keep them out of the settings entry chunk
const Recurring = lazy(() => import('@/features/recurring/components/Recurring'));
const PredictionReview = lazy(
  () => import('@/features/predictionReview/components/PredictionReview'),
);
const LoanOverview = lazy(() => import('@/features/loans/components/LoanOverview'));
const Activity = lazy(() => import('@/features/pipeline/components/Activity'));
import styles from './Settings.module.css';

/* ────────────────────────────────────────────────────────────── */
/*  Types                                                        */
/* ────────────────────────────────────────────────────────────── */

interface ConnectedProvider {
  providerType: string;
  providerId: string;
  email?: string;
  name?: string;
  picture?: string;
  lastGmailSync?: string;
}

interface CurrentUser {
  id: string;
  email: string;
  name: string;
  picture?: string;
  providers: ConnectedProvider[];
}

type SectionId = 'general' | 'loans' | 'recurring' | 'activity' | 'review' | 'tags' | 'ai';

interface SectionDef {
  id: SectionId;
  label: string;
  icon: ReactNode;
  description: string;
}

const SECTIONS: SectionDef[] = [
  {
    id: 'general',
    label: 'General',
    icon: <GearSix size={18} />,
    description: 'Providers, account, and preferences',
  },
  {
    id: 'loans',
    label: 'Loans',
    icon: <Banknote size={18} />,
    description: 'Payoff progress and simulator per loan account',
  },
  {
    id: 'recurring',
    label: 'Recurring',
    icon: <ArrowsClockwise size={18} />,
    description: 'Scheduled transactions created automatically',
  },
  {
    id: 'activity',
    label: 'Activity',
    icon: <Pulse size={18} />,
    description: 'Email pipeline runs and retries',
  },
  {
    id: 'review',
    label: 'Prediction Review',
    icon: <ListChecks size={18} />,
    description: 'Confirm or correct AI-classified transactions',
  },
  {
    id: 'tags',
    label: 'Tags',
    icon: <Tag size={18} />,
    description: 'Create, rename, and recolor transaction tags',
  },
  {
    id: 'ai',
    label: 'AI & Predictions',
    icon: <Robot size={18} />,
    description: 'Models, classifiers, API keys, and prediction insights',
  },
];

/* ────────────────────────────────────────────────────────────── */
/*  Components                                                   */
/* ────────────────────────────────────────────────────────────── */

function SidebarNav({
  active,
  onChange,
  badges,
}: {
  active: SectionId;
  onChange: (id: SectionId) => void;
  badges?: Partial<Record<SectionId, number>>;
}) {
  return (
    <nav className={styles.sidebar} role="tablist" aria-label="Settings sections">
      {SECTIONS.map((section) => {
        const badge = badges?.[section.id];
        return (
          <button
            key={section.id}
            type="button"
            role="tab"
            aria-selected={active === section.id}
            className={`${styles.navItem} ${active === section.id ? styles.navItemActive : ''}`}
            onClick={() => onChange(section.id)}>
            <span className={styles.navIcon}>{section.icon}</span>
            <span className={styles.navText}>
              <strong>{section.label}</strong>
              <span>{section.description}</span>
            </span>
            {badge ? <span className={styles.navBadge}>{badge}</span> : null}
          </button>
        );
      })}
    </nav>
  );
}

const SECTION_IDS = SECTIONS.map((section) => section.id);

function isSectionId(value: string | null): value is SectionId {
  return value !== null && (SECTION_IDS as string[]).includes(value);
}

export default function Settings() {
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();

  // ?section= keeps sections linkable now that they aren't routes of their own
  const sectionParam = searchParams.get('section');
  const activeSection: SectionId = isSectionId(sectionParam) ? sectionParam : 'general';

  // switching sections drops any section-scoped params (e.g. loans' ?account=)
  const handleSectionChange = (id: SectionId) => {
    setSearchParams(id === 'general' ? {} : { section: id }, { replace: true });
  };

  const activeRunCount = useAppSelector(selectActivePipelineRunCount);

  useEffect(() => {
    apiClient
      .get<CurrentUser>('auth/users/me')
      .then(setUser)
      .catch(() => setUser(null));
  }, []);

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1>Settings</h1>
        </div>
      </div>

      <div className={styles.settingsLayout}>
        <SidebarNav
          active={activeSection}
          onChange={handleSectionChange}
          badges={{ activity: activeRunCount }}
        />

        <div className={styles.sectionContent}>
          {activeSection === 'general' && <GeneralSettings user={user} />}
          {activeSection === 'loans' && (
            <Suspense fallback={<div>Loading…</div>}>
              <LoanOverview />
            </Suspense>
          )}
          {activeSection === 'recurring' && (
            <Suspense fallback={<div>Loading…</div>}>
              <Recurring />
            </Suspense>
          )}
          {activeSection === 'activity' && (
            <Suspense fallback={<div>Loading…</div>}>
              <Activity />
            </Suspense>
          )}
          {activeSection === 'review' && (
            <Suspense fallback={<div>Loading…</div>}>
              <PredictionReview />
            </Suspense>
          )}
          {activeSection === 'tags' && <TagSettings />}
          {activeSection === 'ai' && <AISettings />}
        </div>
      </div>
    </section>
  );
}
