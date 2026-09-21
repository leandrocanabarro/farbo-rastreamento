import type { ReactNode } from 'react';
import styles from './Badge.module.css';

export type BadgeTone = 'success' | 'warning' | 'danger' | 'accent' | 'neutral';

interface BadgeProps {
  tone?: BadgeTone;
  dot?: boolean;
  /** Pisca discretamente — usado no estado "online". */
  pulse?: boolean;
  children: ReactNode;
  title?: string;
}

export function Badge({ tone = 'neutral', dot = false, pulse = false, children, title }: BadgeProps) {
  return (
    <span className={`${styles.badge} ${styles[tone]}`} title={title}>
      {dot && <span className={`${styles.dot} ${pulse ? styles.pulse : ''}`} aria-hidden="true" />}
      {children}
    </span>
  );
}
