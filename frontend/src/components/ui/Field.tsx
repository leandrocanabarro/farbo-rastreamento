import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from 'react';
import { useId } from 'react';

import styles from './Field.module.css';

interface FieldProps {
  label: string;
  hint?: ReactNode;
  error?: string;
  children: (id: string) => ReactNode;
}

export function Field({ label, hint, error, children }: FieldProps) {
  const id = useId();
  return (
    <div className={styles.field}>
      <label className={styles.label} htmlFor={id}>
        {label}
      </label>
      {children(id)}
      {hint && <span className={styles.hint}>{hint}</span>}
      {error && <span className={styles.error}>{error}</span>}
    </div>
  );
}

export function TextField({
  label,
  hint,
  error,
  ...rest
}: { label: string; hint?: ReactNode; error?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <Field label={label} hint={hint} error={error}>
      {(id) => <input id={id} className={styles.input} {...rest} />}
    </Field>
  );
}

export function SelectField({
  label,
  hint,
  error,
  children,
  ...rest
}: {
  label: string;
  hint?: ReactNode;
  error?: string;
  children: ReactNode;
} & SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <Field label={label} hint={hint} error={error}>
      {(id) => (
        <select id={id} className={styles.select} {...rest}>
          {children}
        </select>
      )}
    </Field>
  );
}

export const fieldStyles = styles;
