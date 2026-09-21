import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';

import styles from './Toast.module.css';

type ToastTone = 'success' | 'error' | 'warning' | 'info';

interface Toast {
  id: number;
  tone: ToastTone;
  title: string;
  description?: string;
}

interface ToastContextValue {
  notify: (toast: Omit<Toast, 'id'>) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

/** Avisos que somem sozinhos; erros ficam mais tempo na tela. */
const DURATIONS: Record<ToastTone, number> = {
  success: 4000,
  info: 4000,
  warning: 7000,
  error: 9000,
};

let nextId = 1;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const dismiss = useCallback((id: number) => {
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }, []);

  const notify = useCallback(
    (toast: Omit<Toast, 'id'>) => {
      const id = nextId++;
      setToasts((current) => [...current, { ...toast, id }]);
      window.setTimeout(() => dismiss(id), DURATIONS[toast.tone]);
    },
    [dismiss],
  );

  const value = useMemo(() => ({ notify }), [notify]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className={styles.container} role="status" aria-live="polite">
        {toasts.map((toast) => (
          <div key={toast.id} className={`${styles.toast} ${styles[toast.tone]}`}>
            <div className={styles.content}>
              <div className={styles.title}>{toast.title}</div>
              {toast.description && <div className={styles.description}>{toast.description}</div>}
            </div>
            <button
              type="button"
              className={styles.close}
              onClick={() => dismiss(toast.id)}
              aria-label="Fechar aviso"
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error('useToast precisa estar dentro de ToastProvider');
  }
  return context;
}
