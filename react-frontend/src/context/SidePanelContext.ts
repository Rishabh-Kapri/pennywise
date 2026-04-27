import { createContext, useContext, type ReactNode } from 'react';

export interface SidePanelContextType {
  sidePanelContent: ReactNode;
  setSidePanelContent: (content: ReactNode) => void;
}

export const SidePanelContext = createContext<SidePanelContextType | null>(null);

export function useSidePanel() {
  const context = useContext(SidePanelContext);
  if (!context) {
    throw new Error('useSidePanel must be used within a SidePanelProvider');
  }
  return context;
}
