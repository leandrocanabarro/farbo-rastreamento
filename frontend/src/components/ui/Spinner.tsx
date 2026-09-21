import styles from './Spinner.module.css';

export function Spinner({ label, inline = false }: { label?: string; inline?: boolean }) {
  return (
    <div className={`${styles.wrapper} ${inline ? styles.inline : ''}`}>
      <div className={styles.spinner} aria-hidden="true" />
      {label && <span>{label}</span>}
      <span className="visually-hidden">Carregando</span>
    </div>
  );
}
