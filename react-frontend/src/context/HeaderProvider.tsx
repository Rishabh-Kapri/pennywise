import React, { useState, type ReactNode } from 'react';
import { HeaderContext, type HeaderContextType } from './HeaderContext';

export const HeaderProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
  const [headerContent, setHeaderContent] = useState<ReactNode>(null);

  const value: HeaderContextType = { headerContent, setHeaderContent };

  return (
    <HeaderContext.Provider value={value}>{children}</HeaderContext.Provider>
  );
};
