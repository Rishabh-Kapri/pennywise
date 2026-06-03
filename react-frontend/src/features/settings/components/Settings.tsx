import { Envelope as Mail, Warning } from '@phosphor-icons/react';
import { useEffect, useState } from 'react';
import { useAppDispatch } from '@/app/hooks';
import { logout } from '@/features/auth';
import { Modal } from '@/components/common/Modal';
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
  const dispatch = useAppDispatch();
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [confirmEmail, setConfirmEmail] = useState('');
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    apiClient
      .get<CurrentUser>('auth/users/me')
      .then(setUser)
      .catch(() => setUser(null));
  }, []);

  const handleDeleteAccount = async (e: React.FormEvent) => {
    e.preventDefault();
    if (confirmEmail !== user?.email) {
      return;
    }

    setIsDeleting(true);
    setDeleteError(null);
    try {
      await apiClient.delete('auth/users/me');
      await dispatch(logout()).unwrap();
    } catch (err: unknown) {
      console.error('Failed to delete account:', err);
      const errorMessage =
        err instanceof Error ? err.message : 'Failed to delete account. Please try again.';
      setDeleteError(errorMessage);
      setIsDeleting(false);
    }
  };

  const closeDeleteModal = () => {
    setIsDeleteModalOpen(false);
    setConfirmEmail('');
    setDeleteError(null);
  };

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

      <div className={`${styles.card} ${styles.dangerZone}`}>
        <div className={styles.cardHeader}>
          <h2>Danger Zone</h2>
        </div>
        <div className={styles.dangerContent}>
          <div className={styles.dangerInfo}>
            <strong>Delete account</strong>
            <span>Permanently delete your account and all associated budgets, transactions, and data. This action cannot be undone.</span>
          </div>
          <button
            type="button"
            className={styles.deleteButton}
            onClick={() => setIsDeleteModalOpen(true)}>
            Delete Account
          </button>
        </div>
      </div>

      <Modal isOpen={isDeleteModalOpen} onClose={closeDeleteModal}>
        <form onSubmit={handleDeleteAccount} className={styles.deleteForm}>
          <div className={styles.modalHeader}>
            <Warning size={24} className={styles.warningIcon} weight="fill" />
            <h3>Delete Account</h3>
          </div>
          
          <div className={styles.modalDescription}>
            <p>This action is <strong>permanent</strong> and <strong>cannot be undone</strong>.</p>
            <p>All your budgets, transactions, categories, and account configurations will be deleted forever.</p>
          </div>

          <div className={styles.confirmPrompt}>
            <label htmlFor="confirm-email-input">
              Please type your email <strong>{user?.email}</strong> to confirm:
            </label>
            <input
              id="confirm-email-input"
              type="email"
              required
              placeholder={user?.email}
              value={confirmEmail}
              disabled={isDeleting}
              onChange={(e) => setConfirmEmail(e.target.value)}
              className={styles.confirmInput}
            />
          </div>

          {deleteError && (
            <div className={styles.errorMessage}>
              {deleteError}
            </div>
          )}

          <div className={styles.modalActions}>
            <button
              type="button"
              className={styles.cancelButton}
              onClick={closeDeleteModal}
              disabled={isDeleting}>
              Cancel
            </button>
            <button
              type="submit"
              className={styles.confirmDeleteButton}
              disabled={isDeleting || confirmEmail !== user?.email}>
              {isDeleting ? 'Deleting...' : 'Permanently Delete'}
            </button>
          </div>
        </form>
      </Modal>
    </section>
  );
}
