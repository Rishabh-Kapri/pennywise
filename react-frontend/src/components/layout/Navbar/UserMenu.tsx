import { GearSix as Settings, SignOut, UserCircle } from '@phosphor-icons/react';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAppDispatch } from '@/app/hooks';
import { logout } from '@/features/auth/store/authSlice';
import { useAccountProfile } from '@/features/auth/hooks/useAccountProfile';
import styles from './Navbar.module.css';

export function UserMenu() {
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const user = useAccountProfile();
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

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
          <button
            type="button"
            className={`${styles.dropdownAction} ${styles.dropdownActionDanger}`}
            role="menuitem"
            onClick={() => {
              setIsOpen(false);
              dispatch(logout());
            }}>
            <SignOut size={16} strokeWidth={1.8} />
            <span>Log out</span>
          </button>
        </div>
      )}
    </div>
  );
}
