import { useState } from 'react';
import styles from './Sharing.module.css';

/** A member's Google picture, or their initial when there's none or it fails to load. */
export function Avatar({ name, email, picture }: { name: string; email: string; picture?: string }) {
  const [broken, setBroken] = useState(false);
  const initial = (name || email || '?').trim().charAt(0).toUpperCase();
  return (
    <span className={styles.avatar} aria-hidden="true">
      {picture && !broken ? (
        <img src={picture} alt="" referrerPolicy="no-referrer" onError={() => setBroken(true)} />
      ) : (
        initial
      )}
    </span>
  );
}
