import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { flushSync } from 'react-dom';
import { ThemeContext, type Theme } from './ThemeContext';

const storageKey = 'pennywise-theme';
function readTheme(): Theme {
  try {
    return (localStorage.getItem(storageKey) ?? localStorage.getItem('pennywise-home-theme')) === 'light'
      ? 'light'
      : 'dark';
  } catch {
    return 'dark';
  }
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(readTheme);
  const transitioning = useRef(false);

  const toggleTheme = () => {
    // Keep repeated clicks from overlapping document snapshots.
    if (transitioning.current) return;
    const updateTheme = () => setTheme((current) => (current === 'dark' ? 'light' : 'dark'));
    if (!document.startViewTransition || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      updateTheme();
      return;
    }

    const root = document.documentElement;
    root.dataset.themeTransition = 'fade';
    transitioning.current = true;

    const cleanup = () => {
      delete root.dataset.themeTransition;
      transitioning.current = false;
    };
    try {
      const transition = document.startViewTransition(() => flushSync(updateTheme));
      // A hidden tab can skip the animation; it must still apply the new theme.
      void transition.ready.catch(() => {});
      void transition.finished.then(cleanup, cleanup);
    } catch {
      cleanup();
      updateTheme();
    }
  };

  useLayoutEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.classList.toggle('dark', theme === 'dark');
    document.documentElement.classList.toggle('light', theme === 'light');
    try {
      localStorage.setItem(storageKey, theme);
    } catch {
      /* Storage may be disabled. */
    }
  }, [theme]);
  useEffect(() => {
    const syncTheme = (event: StorageEvent) => {
      if (event.key === storageKey || event.key === null) setTheme(readTheme());
    };
    window.addEventListener('storage', syncTheme);
    return () => window.removeEventListener('storage', syncTheme);
  }, []);
  return <ThemeContext.Provider value={{ theme, toggleTheme }}>{children}</ThemeContext.Provider>;
}
