export type DropdownVariant = 'inline' | 'form';

export interface TransactionDropdownProps {
  value: string;
  onClick: (id: string, name: string) => void;
  autoFocus?: boolean;
  variant?: DropdownVariant;
}
