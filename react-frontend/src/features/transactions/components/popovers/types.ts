/** inline: in a table row; form: labelled form field; ghost: borderless until hovered (detail panel rows). */
export type DropdownVariant = 'inline' | 'form' | 'ghost';

export interface TransactionDropdownProps {
  value: string;
  onClick: (id: string, name: string) => void;
  autoFocus?: boolean;
  variant?: DropdownVariant;
  multiple?: boolean;
  selectedIds?: string[];
  onChangeMultiple?: (ids: string[], names: string[]) => void;
  /** Text shown while nothing is picked. */
  placeholder?: string;
}
