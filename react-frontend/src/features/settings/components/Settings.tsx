import { Mail } from 'lucide-react';
import { useEffect, useState } from 'react';
import { apiClient } from '@/utils';
import styles from './Settings.module.css';

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

export default function Settings() {
  const [user, setUser] = useState<CurrentUser | null>(null);

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

      <div className={styles.card}>
        <div className={styles.cardHeader}>
          <h2>Connected providers</h2>
          <span>{user?.providers.length ?? 0} connected</span>
        </div>

        {user?.providers.length ? (
          <div className={styles.providerList}>
            {user.providers.map((provider) => (
              <article key={`${provider.providerType}-${provider.providerId}`} className={styles.providerCard}>
                {provider.picture ? (
                  <img src={provider.picture} alt="" className={styles.providerImage} />
                ) : (
                  <div className={styles.providerFallback}>
                    <Mail size={18} strokeWidth={1.8} />
                  </div>
                )}
                <div className={styles.providerInfo}>
                  <strong>{provider.providerType}</strong>
                  <span>{provider.email || provider.name || provider.providerId}</span>
                  {provider.lastGmailSync && (
                    <small>Last Gmail sync {new Date(provider.lastGmailSync).toLocaleDateString()}</small>
                  )}
                </div>
              </article>
            ))}
          </div>
        ) : (
          <p className={styles.empty}>No connected providers found.</p>
        )}
      </div>
    </section>
  );
}
