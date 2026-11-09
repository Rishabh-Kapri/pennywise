import {
  ArrowsClockwise,
  Files,
  FileMagnifyingGlass,
  GearSix,
  Envelope,
  ListChecks,
  MagnifyingGlass,
  Money as Banknote,
  EnvelopeSimple,
  Pulse,
  Sparkle,
  Tag,
  TreeStructure,
  type IconProps,
} from '@phosphor-icons/react';
import {
  lazy,
  Suspense,
  useEffect,
  useRef,
  useState,
  type ComponentType,
  type KeyboardEvent,
} from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { selectActivePipelineRunCount } from '@/features/pipeline/store';
import { apiClient } from '@/utils';
import { GeneralSettings } from './GeneralSettings';
import { AISettings } from './AISettings';
import { TagSettings } from './TagSettings';
import { GmailSettings } from './GmailSettings';

// heavier, data-driven sections: keep them out of the settings entry chunk
const Recurring = lazy(() => import('@/features/recurring/components/Recurring'));
const PredictionReview = lazy(
  () => import('@/features/predictionReview/components/PredictionReview'),
);
const LoanOverview = lazy(() => import('@/features/loans/components/LoanOverview'));
const Activity = lazy(() => import('@/features/pipeline/components/Activity'));
const Jobs = lazy(() => import('@/features/jobs/components/Jobs'));
const DocumentLibrary = lazy(() => import('@/features/documents/components/DocumentLibrary'));
const StatementReconcile = lazy(() => import('@/features/reconcile/components/StatementReconcile'));
import { MailSettings } from './MailSettings';
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
}

interface CurrentUser {
  id: string;
  email: string;
  name: string;
  picture?: string;
  providers: ConnectedProvider[];
}

type SectionId =
  | 'general'
  | 'gmail'
  | 'loans'
  | 'recurring'
  | 'documents'
  | 'reconcile'
  | 'activity'
  | 'mail'
  | 'jobs'
  | 'review'
  | 'tags'
  | 'ai';

type GroupId = 'account' | 'budget' | 'automation';

interface SectionDef {
  id: SectionId;
  group: GroupId;
  label: string;
  icon: ComponentType<IconProps>;
  description: string;
  /** Extra words the nav search matches on. */
  keywords?: string;
  /** The section renders its own title, so the page header is left out. */
  ownHeader?: boolean;
}

const GROUPS: { id: GroupId; label: string }[] = [
  { id: 'account', label: 'Account' },
  { id: 'budget', label: 'Budget' },
  { id: 'automation', label: 'Automation' },
];

const SECTIONS: SectionDef[] = [
  {
    id: 'general',
    group: 'account',
    label: 'General',
    icon: GearSix,
    description: 'Your profile and the Google account you sign in with.',
    keywords: 'profile google gmail delete account',
  },
  {
    id: 'gmail',
    label: 'Gmail',
    icon: <Envelope size={18} />,
    description: 'Connection health and automatic email imports',
  },
  {
    id: 'loans',
    group: 'budget',
    label: 'Loans',
    icon: Banknote,
    description: 'Payoff progress and a payoff simulator for each loan account.',
    keywords: 'emi mortgage debt interest',
  },
  {
    id: 'recurring',
    ownHeader: true,
    group: 'budget',
    label: 'Recurring',
    icon: ArrowsClockwise,
    description: 'Scheduled transactions that are created for you automatically.',
    keywords: 'subscription schedule repeat',
  },
  {
    id: 'documents',
    ownHeader: true,
    group: 'budget',
    label: 'Documents',
    icon: Files,
    description: 'Every receipt and scan attached to transactions in this budget.',
    keywords: 'receipt scan attachment',
  },
  {
    id: 'reconcile',
    ownHeader: true,
    group: 'budget',
    label: 'Reconcile',
    icon: FileMagnifyingGlass,
    description: 'Match bank statement PDFs against your transactions.',
    keywords: 'statement pdf import',
  },
  {
    id: 'tags',
    ownHeader: true,
    group: 'budget',
    label: 'Tags',
    icon: Tag,
    description: 'Create, rename and recolour transaction tags.',
    keywords: 'label colour color',
  },
  {
    id: 'ai',
    group: 'automation',
    label: 'AI & Predictions',
    icon: Sparkle,
    description: 'Your AI providers, how well predictions are doing, and API keys.',
    keywords: 'openai anthropic openrouter ollama model key llm classifier',
  },
  {
    id: 'review',
    ownHeader: true,
    group: 'automation',
    label: 'Prediction Review',
    icon: ListChecks,
    description: 'Confirm or correct transactions the AI classified.',
    keywords: 'approve correct classify',
  },
  {
    id: 'mail',
    group: 'automation',
    label: 'Email sync',
    icon: EnvelopeSimple,
    description: 'The Gmail accounts Pennywise reads bank alerts from.',
    keywords: 'gmail mailbox connect inbox permissions',
  },
  {
    id: 'activity',
    group: 'automation',
    label: 'Activity',
    icon: Pulse,
    description: 'Email pipeline runs, what each email became, and retries.',
    keywords: 'pipeline email gmail runs retry',
  },
  {
    id: 'jobs',
    group: 'automation',
    label: 'Jobs',
    icon: TreeStructure,
    description: 'What happened to each email, and why anything stopped.',
    keywords: 'queue worker retry failed error mailbox message',
  },
];

/* ────────────────────────────────────────────────────────────── */
/*  Components                                                   */
/* ────────────────────────────────────────────────────────────── */

function matches(section: SectionDef, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return `${section.label} ${section.description} ${section.keywords ?? ''}`.toLowerCase().includes(q);
}

function SidebarNav({
  active,
  onChange,
  badges,
}: {
  active: SectionId;
  onChange: (id: SectionId) => void;
  badges?: Partial<Record<SectionId, number>>;
}) {
  const [query, setQuery] = useState('');
  const visible = SECTIONS.filter((section) => matches(section, query));
  const tabRefs = useRef<Partial<Record<SectionId, HTMLButtonElement | null>>>({});

  // Arrow keys move between tabs (and select them), as a vertical tablist should.
  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>, id: SectionId) => {
    const keys = ['ArrowDown', 'ArrowUp', 'ArrowRight', 'ArrowLeft', 'Home', 'End'];
    if (!keys.includes(event.key) || visible.length === 0) return;
    event.preventDefault();
    const index = visible.findIndex((section) => section.id === id);
    const forward = event.key === 'ArrowDown' || event.key === 'ArrowRight';
    const next =
      event.key === 'Home'
        ? visible[0]
        : event.key === 'End'
          ? visible[visible.length - 1]
          : visible[(index + (forward ? 1 : -1) + visible.length) % visible.length];
    onChange(next.id);
    tabRefs.current[next.id]?.focus();
  };

  return (
    <aside className={styles.sidebar}>
      <h1 className={styles.title}>Settings</h1>
      <label className={styles.search}>
        <MagnifyingGlass size={15} aria-hidden="true" />
        <input
          type="search"
          value={query}
          placeholder="Find a setting"
          aria-label="Find a setting"
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && visible[0]) onChange(visible[0].id);
            if (event.key === 'Escape') setQuery('');
          }}
        />
      </label>

      <nav className={styles.nav} role="tablist" aria-orientation="vertical" aria-label="Settings sections">
        {GROUPS.map((group) => {
          const sections = visible.filter((section) => section.group === group.id);
          if (sections.length === 0) return null;
          return (
            <div key={group.id} className={styles.navGroup} role="presentation">
              <span className={styles.navGroupLabel}>{group.label}</span>
              {sections.map((section) => {
                const badge = badges?.[section.id];
                const isActive = active === section.id;
                const Icon = section.icon;
                return (
                  <button
                    key={section.id}
                    ref={(el) => {
                      tabRefs.current[section.id] = el;
                    }}
                    type="button"
                    role="tab"
                    id={`settings-tab-${section.id}`}
                    aria-selected={isActive}
                    aria-controls="settings-panel"
                    tabIndex={isActive ? 0 : -1}
                    title={section.description}
                    className={`${styles.navItem} ${isActive ? styles.navItemActive : ''}`}
                    onClick={() => onChange(section.id)}
                    onKeyDown={(event) => handleKeyDown(event, section.id)}>
                    <Icon size={18} weight={isActive ? 'fill' : 'regular'} className={styles.navIcon} />
                    <span className={styles.navLabel}>{section.label}</span>
                    {badge ? (
                      <span className={styles.navBadge} aria-label={`${badge} running`}>
                        {badge}
                      </span>
                    ) : null}
                  </button>
                );
              })}
            </div>
          );
        })}
        {visible.length === 0 && <p className={styles.navEmpty}>No settings match "{query}".</p>}
      </nav>
    </aside>
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

  const section = SECTIONS.find((s) => s.id === activeSection) ?? SECTIONS[0];
  const loading = <div className={styles.loading}>Loading…</div>;

  return (
    <section className={styles.page}>
      <SidebarNav
        active={activeSection}
        onChange={handleSectionChange}
        badges={{ activity: activeRunCount }}
      />

      <div
        className={styles.sectionContent}
        id="settings-panel"
        role="tabpanel"
        aria-labelledby={`settings-tab-${activeSection}`}>
        {!section.ownHeader && (
          <header className={styles.sectionHeader}>
            <h2>{section.label}</h2>
            <p>{section.description}</p>
          </header>
        )}

        {activeSection === 'general' && <GeneralSettings user={user} />}
        {activeSection === 'loans' && (
          <Suspense fallback={loading}>
            <LoanOverview />
          </Suspense>
        )}
        {activeSection === 'recurring' && (
          <Suspense fallback={loading}>
            <Recurring />
          </Suspense>
        )}
        {activeSection === 'documents' && (
          <Suspense fallback={loading}>
            <DocumentLibrary />
          </Suspense>
        )}
        {activeSection === 'reconcile' && (
          <Suspense fallback={loading}>
            <StatementReconcile />
          </Suspense>
        )}
        {activeSection === 'mail' && <MailSettings />}
        {activeSection === 'activity' && (
          <Suspense fallback={loading}>
            <Activity />
          </Suspense>
        )}
        {activeSection === 'jobs' && (
          <Suspense fallback={loading}>
            <Jobs />
          </Suspense>
        )}
        {activeSection === 'review' && (
          <Suspense fallback={loading}>
            <PredictionReview />
          </Suspense>
        )}
        {activeSection === 'tags' && <TagSettings />}
        {activeSection === 'ai' && <AISettings />}
      </div>
    </section>
  );
}
