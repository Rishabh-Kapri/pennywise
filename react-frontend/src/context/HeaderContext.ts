import { createContext, useContext, type ReactNode } from 'react';

export interface HeaderContextType {
  headerContent: ReactNode;
  setHeaderContent: (content: ReactNode) => void;
}

export const HeaderContext = createContext<HeaderContextType | null>(null);

export function useHeader() {
  const context = useContext(HeaderContext);
  if (!context) {
    throw new Error('useHeader must be used within a HeaderProvider');
  }
  return context;
}
