import {
  Bank,
  Car,
  Coins,
  CreditCard,
  FirstAid,
  GraduationCap,
  HandCoins,
  House,
  PiggyBank,
  TrendDown,
  type Icon,
} from '@phosphor-icons/react';
import type { Account } from '../../types/account.types';
import { bankLogoUrl, getBankBrand } from '../../bankLogos';
import styles from './BankLogo.module.css';

const typeIcons: Record<string, Icon> = {
  checking: Bank,
  savings: PiggyBank,
  creditCard: CreditCard,
  asset: Coins,
  liability: TrendDown,
  loan: HandCoins,
  mortgage: House,
  autoLoan: Car,
  studentLoan: GraduationCap,
  personalLoan: HandCoins,
  medicalDebt: FirstAid,
};

const iconSizes = { xs: 11, sm: 13, md: 16, lg: 20 } as const;

interface BankLogoProps {
  /** Account name; the bank is guessed from it when `bank` isn't set. */
  name?: string | null;
  /** Stored bank id (`account.bank`). */
  bank?: string | null;
  /** Account type. Credit cards are drawn as an outlined card. */
  type?: Account['type'] | string | null;
  size?: keyof typeof iconSizes;
  className?: string;
}

/**
 * The account's bank logo, or its type icon when the bank isn't known.
 * Credit cards put the logo inside Phosphor's card icon so they read apart
 * from bank accounts, which show the bare logo.
 */
export function BankLogo({ name, bank: bankId, type, size = 'md', className = '' }: BankLogoProps) {
  const bank = getBankBrand(name, bankId);
  const sizeClass = styles[size];

  if (type === 'creditCard') {
    // viewBox crops Phosphor's 256 canvas to the card (x16-240, y48-208); the logo is
    // drawn in the same coordinates, in the open area left of the icon's two dashes.
    return (
      <CreditCard
        className={`${styles.card} ${sizeClass} ${className}`}
        viewBox="16 48 224 160"
        weight="regular"
        aria-hidden="true">
        {bank && <image href={bankLogoUrl(bank)} x="40" y="113" width="70" height="70" />}
      </CreditCard>
    );
  }

  if (bank) {
    return (
      <span className={`${styles.logo} ${sizeClass} ${className}`} aria-hidden="true">
        <img src={bankLogoUrl(bank)} alt="" draggable={false} />
      </span>
    );
  }

  const IconComponent = (type && typeIcons[type]) || Bank;
  return (
    <span className={`${styles.tile} ${sizeClass} ${className}`} data-type={type ?? undefined} aria-hidden="true">
      <IconComponent size={iconSizes[size]} weight="duotone" />
    </span>
  );
}
