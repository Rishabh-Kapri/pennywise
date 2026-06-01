import React, { useState, type ReactNode } from 'react';
import { SidePanelContext } from './SidePanelContext';

export const SidePanelProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
  const [sidePanelContent, setSidePanelContent] = useState<ReactNode>(null);

  const value = { sidePanelContent, setSidePanelContent };

  return (
    <SidePanelContext.Provider value={value}>{children}</SidePanelContext.Provider>
  );
};
