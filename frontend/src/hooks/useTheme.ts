import { useCallback, useEffect, useState } from 'react';

type Theme = 'dark' | 'light';

const STORAGE_KEY = 'tracker.theme';

function readStoredTheme(): Theme | null {
  try {
    const value = window.localStorage.getItem(STORAGE_KEY);
    return value === 'dark' || value === 'light' ? value : null;
  } catch {
    return null;
  }
}

/**
 * Tema do painel.
 *
 * O escuro é o padrão; a preferência escolhida fica no navegador de quem usa,
 * sem ida ao servidor.
 */
export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => readStoredTheme() ?? 'dark');

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    try {
      window.localStorage.setItem(STORAGE_KEY, theme);
    } catch {
      /* storage bloqueado: o tema vale só para esta sessão */
    }
  }, [theme]);

  const toggle = useCallback(() => {
    setTheme((current) => (current === 'dark' ? 'light' : 'dark'));
  }, []);

  return { theme, toggle };
}
