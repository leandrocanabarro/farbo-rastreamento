import type { ButtonHTMLAttributes, ReactNode } from 'react';
import styles from './Button.module.css';

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost';
type Size = 'small' | 'medium' | 'large';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  block?: boolean;
  loading?: boolean;
  icon?: ReactNode;
}

export function Button({
  variant = 'secondary',
  size = 'medium',
  block = false,
  loading = false,
  icon,
  children,
  disabled,
  className,
  ...rest
}: ButtonProps) {
  const classes = [
    styles.button,
    styles[variant],
    size !== 'medium' ? styles[size] : '',
    block ? styles.block : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <button className={classes} disabled={disabled || loading} {...rest}>
      {loading ? <span className={styles.spinner} aria-hidden="true" /> : icon}
      {children}
    </button>
  );
}
