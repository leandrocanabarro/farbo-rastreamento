import { useState } from 'react';
import type { FormEvent } from 'react';
import { Navigate } from 'react-router-dom';

import { Button } from '@/components/ui/Button';
import { TextField } from '@/components/ui/Field';
import { Spinner } from '@/components/ui/Spinner';
import { useAuth } from '@/stores/AuthContext';

import styles from './LoginPage.module.css';

export function LoginPage() {
  const { user, loading, login } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  if (loading) {
    return <Spinner label="Verificando sessão" />;
  }
  if (user) {
    return <Navigate to="/" replace />;
  }

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      await login(email, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'não foi possível entrar');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.card}>
        <div className={styles.brand}>
          <span className={styles.mark} aria-hidden="true">
            ◉
          </span>
          <div>
            <h1 className={styles.title}>Rastreamento</h1>
            <span className={styles.subtitle}>Acesso ao painel</span>
          </div>
        </div>

        <form className={styles.form} onSubmit={onSubmit}>
          {error && <div className={styles.error}>{error}</div>}

          <TextField
            label="E-mail"
            type="email"
            autoComplete="username"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />

          <TextField
            label="Senha"
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />

          <Button type="submit" variant="primary" size="large" block loading={submitting}>
            Entrar
          </Button>
        </form>
      </div>
    </div>
  );
}
