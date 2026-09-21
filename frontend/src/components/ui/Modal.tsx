import { useEffect, useRef } from 'react';
import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';

import styles from './Modal.module.css';

interface ModalProps {
  open: boolean;
  title: ReactNode;
  icon?: ReactNode;
  wide?: boolean;
  onClose: () => void;
  footer?: ReactNode;
  children: ReactNode;
}

export function Modal({ open, title, icon, wide = false, onClose, footer, children }: ModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null);

  // Esc fecha; o foco vai para o diálogo para quem navega pelo teclado.
  useEffect(() => {
    if (!open) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKeyDown);
    dialogRef.current?.focus();

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.body.style.overflow = previousOverflow;
    };
  }, [open, onClose]);

  if (!open) return null;

  return createPortal(
    <div
      className={styles.backdrop}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div
        ref={dialogRef}
        className={`${styles.dialog} ${wide ? styles.wide : ''}`}
        role="dialog"
        aria-modal="true"
        tabIndex={-1}
      >
        <header className={styles.header}>
          {icon && (
            <span className={styles.icon} aria-hidden="true">
              {icon}
            </span>
          )}
          <h2 className={styles.title}>{title}</h2>
        </header>
        <div className={styles.body}>{children}</div>
        {footer && <footer className={styles.footer}>{footer}</footer>}
      </div>
    </div>,
    document.body,
  );
}
