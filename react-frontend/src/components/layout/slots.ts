import { createContext, useContext, type ReactNode } from 'react';

/**
 * Places a page can fill in the shared `Layout`: the header bar above it and
 * the side panel beside it. `Layout` provides both. The setters live in their
 * own context so a page that fills a slot doesn't re-render when it changes.
 */
export interface LayoutSlotSetters {
  setHeaderContent: (content: ReactNode) => void;
  setSidePanelContent: (content: ReactNode) => void;
}

export interface LayoutSlotContent {
  headerContent: ReactNode;
  sidePanelContent: ReactNode;
}

export const LayoutSlotSettersContext = createContext<LayoutSlotSetters | null>(null);
export const LayoutSlotContentContext = createContext<LayoutSlotContent>({
  headerContent: null,
  sidePanelContent: null,
});

function useSlotSetters(): LayoutSlotSetters {
  const setters = useContext(LayoutSlotSettersContext);
  if (!setters) throw new Error('layout slots are only available inside Layout');
  return setters;
}

export function useHeader() {
  return { setHeaderContent: useSlotSetters().setHeaderContent };
}

export function useSidePanel() {
  return { setSidePanelContent: useSlotSetters().setSidePanelContent };
}
