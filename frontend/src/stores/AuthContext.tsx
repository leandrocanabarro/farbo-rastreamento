import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';

import { authApi } from '@/api/resources';
import { onUnauthorized, tokens } from '@/api/client';
import type { User } from '@/types';

interface AuthContextValue {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  canSendCommands: boolean;
  canManage: boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  // Recupera a sessão ao abrir o app: se o refresh token ainda vale, o
  // cliente renova sozinho na primeira chamada.
  useEffect(() => {
    let active = true;

    (async () => {
      if (!tokens.accessToken && !tokens.refreshToken) {
        setLoading(false);
        return;
      }
      try {
        const me = await authApi.me();
        if (active) setUser(me);
      } catch {
        tokens.clear();
      } finally {
        if (active) setLoading(false);
      }
    })();

    return () => {
      active = false;
    };
  }, []);

  // O cliente avisa quando a renovação falhou de vez.
  useEffect(() => onUnauthorized(() => setUser(null)), []);

  const login = useCallback(async (email: string, password: string) => {
    const result = await authApi.login(email, password);
    tokens.save(result);
    setUser(result.user);
  }, []);

  const logout = useCallback(async () => {
    const refreshToken = tokens.refreshToken;
    if (refreshToken) {
      try {
        await authApi.logout(refreshToken);
      } catch {
        /* a sessão local termina de qualquer forma */
      }
    }
    tokens.clear();
    setUser(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading,
      login,
      logout,
      canSendCommands: user?.role === 'admin' || user?.role === 'operator',
      canManage: user?.role === 'admin',
    }),
    [user, loading, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth precisa estar dentro de AuthProvider');
  }
  return context;
}
