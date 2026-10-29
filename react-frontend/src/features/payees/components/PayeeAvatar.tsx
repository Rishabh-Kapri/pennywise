import type { CSSProperties } from 'react';
import { ArrowsLeftRight } from '@phosphor-icons/react';
import { payeeHue, payeeInitials } from '../payeeUtils';
import styles from './Payees.module.css';

export function PayeeAvatar({ name, transfer, size = 'md' }: { name: string; transfer?: boolean; size?: 'md' | 'lg' }) {
  return (
    <span
      className={`${styles.avatar} ${size === 'lg' ? styles.avatarLarge : ''} ${transfer ? styles.avatarTransfer : ''}`}
      style={transfer ? undefined : ({ '--avatar-hue': payeeHue(name) } as CSSProperties)}
      aria-hidden="true">
      {transfer ? <ArrowsLeftRight size={size === 'lg' ? 20 : 15} /> : payeeInitials(name)}
    </span>
  );
}
