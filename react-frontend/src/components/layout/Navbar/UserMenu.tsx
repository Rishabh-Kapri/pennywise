import { Settings, UserCircle } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiClient } from '@/utils';
import styles from './Navbar.module.css';

interface CurrentAuthProvider {
  providerType: string;
  providerId: string;
  picture?: string;
}

interface CurrentAuthUser {
  id: string;
  email: string;
  name: string;
  picture?: string;
  providers: CurrentAuthProvider[];
}

export function UserMenu() {
  const navigate = useNavigate();
  const [user, setUser] = useState<CurrentAuthUser | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    apiClient
      .get<CurrentAuthUser>('auth/users/me')
      .then(setUser)
      .catch(() => setUser(null));
  }, []);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener('pointerdown', handlePointerDown);
    return () => document.removeEventListener('pointerdown', handlePointerDown);
  }, [isOpen]);

  const picture =
    user?.picture ||
    user?.providers.find((provider) => provider.picture)?.picture;

  return (
    <div className={styles.userMenu} ref={menuRef}>
      <button
        type="button"
        className={styles.profileButton}
        aria-label="Open user menu"
        aria-haspopup="menu"
        aria-expanded={isOpen}
        onClick={() => setIsOpen((current) => !current)}>
        {picture ? (
          <span
            className={styles.profileImage}
            style={{ backgroundImage: `url(${picture})` }}
            aria-hidden="true"
          />
        ) : (
          <UserCircle size={28} strokeWidth={1.5} />
        )}
      </button>

      {isOpen && (
        <div className={styles.userDropdown} role="menu">
          <div className={styles.userSummary}>
            {picture ? (
              <span
                className={styles.dropdownImage}
                style={{ backgroundImage: `url(${picture})` }}
                aria-hidden="true"
              />
            ) : (
              <UserCircle size={36} strokeWidth={1.5} />
            )}
            <div className={styles.userText}>
              <strong>{user?.name || 'User'}</strong>
              <span>{user?.email || 'Signed in'}</span>
            </div>
          </div>
          <button
            type="button"
            className={styles.dropdownAction}
            role="menuitem"
            onClick={() => {
              setIsOpen(false);
              navigate('/settings');
            }}>
            <Settings size={16} strokeWidth={1.8} />
            <span>Settings</span>
          </button>
        </div>
      )}
    </div>
  );
}
