import { NavLink, Outlet } from 'react-router-dom';

import { useRealtime } from '@/hooks/useRealtime';
import { useAuth } from '@/stores/AuthContext';
import { useTheme } from '@/hooks/useTheme';

import styles from './AppShell.module.css';

const ROLE_LABELS: Record<string, string> = {
  admin: 'Administrador',
  operator: 'Operador',
  viewer: 'Visualização',
};

export function AppShell() {
  const { user, logout, canManage } = useAuth();
  const { connected } = useRealtime();
  const { theme, toggle } = useTheme();

  const navClass = ({ isActive }: { isActive: boolean }) =>
    `${styles.navLink} ${isActive ? styles.navActive : ''}`;

  return (
    <div className={styles.shell}>
      <header className={styles.header}>
        <NavLink to="/" className={styles.brand}>
          <span className={styles.mark} aria-hidden="true">
            ◉
          </span>
          <span className={styles.brandText}>Rastreamento</span>
        </NavLink>

        <nav className={styles.nav}>
          <NavLink to="/" end className={navClass}>
            Painel
          </NavLink>
          <NavLink to="/eventos" className={navClass}>
            Eventos
          </NavLink>
          <NavLink to="/cercas" className={navClass}>
            Cercas
          </NavLink>
          {canManage && (
            <>
              <NavLink to="/dispositivos" className={navClass}>
                Rastreadores
              </NavLink>
              <NavLink to="/diagnostico" className={navClass}>
                Diagnóstico
              </NavLink>
            </>
          )}
        </nav>

        <div className={styles.right}>
          <div
            className={styles.connection}
            title={
              connected
                ? 'Recebendo atualizações em tempo real'
                : 'Sem conexão em tempo real; reconectando'
            }
          >
            <span className={`${styles.dot} ${connected ? styles.dotLive : ''}`} />
            <span>{connected ? 'Ao vivo' : 'Reconectando'}</span>
          </div>

          {user && (
            <div className={styles.user}>
              <span className={styles.userName}>{user.name || user.email}</span>
              <span className={styles.userRole}>{ROLE_LABELS[user.role] ?? user.role}</span>
            </div>
          )}

          <button
            type="button"
            className={styles.themeButton}
            onClick={toggle}
            title="Alternar tema claro/escuro"
          >
            {theme === 'dark' ? '☾' : '☀'}
          </button>

          <button type="button" className={styles.logoutButton} onClick={() => void logout()}>
            Sair
          </button>
        </div>
      </header>

      <main className={styles.content}>
        <Outlet />
      </main>
    </div>
  );
}
