import { useAppSelector } from '@/app/hooks';
import { BankLogo } from './BankLogo';

interface AccountLogoProps {
  accountId?: string | null;
  /** Shown when the account isn't loaded, and used to guess its bank. */
  name?: string | null;
  size?: Parameters<typeof BankLogo>[0]['size'];
}

/** The logo of an account known by id, e.g. a transaction's. */
export function AccountLogo({ accountId, name, size }: AccountLogoProps) {
  const account = useAppSelector((state) => state.accounts.allAccounts.find((acc) => acc.id === accountId));
  return <BankLogo name={name ?? account?.name} bank={account?.bank} type={account?.type} size={size} />;
}
