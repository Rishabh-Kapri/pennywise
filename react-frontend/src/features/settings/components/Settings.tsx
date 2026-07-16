import {
  GearSix,
  Robot,
  Tag,
} from '@phosphor-icons/react';
import { useEffect, useState, type ReactNode } from 'react';
import { apiClient } from '@/utils';
import { GeneralSettings } from './GeneralSettings';
import { AISettings } from './AISettings';
import { TagSettings } from './TagSettings';
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

type SectionId = 'general' | 'tags' | 'ai';

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
}: {
  active: SectionId;
  onChange: (id: SectionId) => void;
}) {
  return (
    <nav className={styles.sidebar} role="tablist" aria-label="Settings sections">
      {SECTIONS.map((section) => (
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
        </button>
      ))}
    </nav>
  );
}

export default function Settings() {
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [activeSection, setActiveSection] = useState<SectionId>('general');

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
        <SidebarNav active={activeSection} onChange={setActiveSection} />

        <div className={styles.sectionContent}>
          {activeSection === 'general' && <GeneralSettings user={user} />}
          {activeSection === 'tags' && <TagSettings />}
          {activeSection === 'ai' && <AISettings />}
        </div>
      </div>
    </section>
  );
}
