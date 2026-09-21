import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { geofencesApi } from '@/api/resources';
import type { GeofenceInput } from '@/api/resources';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { TextField } from '@/components/ui/Field';
import { Modal } from '@/components/ui/Modal';
import { Spinner } from '@/components/ui/Spinner';
import { useToast } from '@/components/ui/Toast';
import { useAuth } from '@/stores/AuthContext';
import { formatDistance } from '@/services/format';

import styles from './Page.module.css';

const EMPTY: GeofenceInput = { name: '', latitude: 0, longitude: 0, radiusMeters: 200 };

export function GeofencesPage() {
  const { canManage } = useAuth();
  const { notify } = useToast();
  const queryClient = useQueryClient();

  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<GeofenceInput>(EMPTY);

  const fences = useQuery({ queryKey: ['geofences'], queryFn: geofencesApi.list });

  const create = useMutation({
    mutationFn: (input: GeofenceInput) => geofencesApi.create(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['geofences'] });
      notify({ tone: 'success', title: 'Cerca criada' });
      setOpen(false);
      setForm(EMPTY);
    },
    onError: (error: Error) =>
      notify({ tone: 'error', title: 'Não foi possível criar', description: error.message }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => geofencesApi.remove(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['geofences'] });
      notify({ tone: 'success', title: 'Cerca removida' });
    },
  });

  return (
    <div className={styles.page}>
      <div className={styles.inner}>
        <header className={styles.header}>
          <div>
            <h1 className={styles.title}>Cercas</h1>
            <p className={styles.description}>
              Áreas circulares avaliadas a cada posição recebida. Entrada e saída viram evento e
              aparecem no histórico do veículo.
            </p>
          </div>
          {canManage && (
            <Button variant="primary" onClick={() => setOpen(true)}>
              Nova cerca
            </Button>
          )}
        </header>

        <Card flush>
          {fences.isLoading ? (
            <Spinner label="Carregando cercas" />
          ) : (fences.data ?? []).length === 0 ? (
            <EmptyState
              icon="◯"
              title="Nenhuma cerca cadastrada"
              description="Crie uma cerca para receber aviso quando o veículo entrar ou sair da área."
            />
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Nome</th>
                    <th>Centro</th>
                    <th>Raio</th>
                    <th>Situação</th>
                    {canManage && <th />}
                  </tr>
                </thead>
                <tbody>
                  {(fences.data ?? []).map((fence) => (
                    <tr key={fence.id}>
                      <td>{fence.name}</td>
                      <td className={styles.mono}>
                        {fence.latitude.toFixed(5)}, {fence.longitude.toFixed(5)}
                      </td>
                      <td>{formatDistance(fence.radiusMeters)}</td>
                      <td>{fence.active ? 'Ativa' : 'Inativa'}</td>
                      {canManage && (
                        <td>
                          <div className={styles.actions}>
                            <Button
                              size="small"
                              variant="ghost"
                              onClick={() => remove.mutate(fence.id)}
                            >
                              Remover
                            </Button>
                          </div>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </div>

      <Modal
        open={open}
        title="Nova cerca"
        onClose={() => setOpen(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setOpen(false)}>
              Cancelar
            </Button>
            <Button
              variant="primary"
              loading={create.isPending}
              onClick={() => create.mutate(form)}
            >
              Criar
            </Button>
          </>
        }
      >
        <div className={styles.form}>
          <TextField
            label="Nome"
            value={form.name}
            onChange={(event) => setForm({ ...form, name: event.target.value })}
          />
          <div className={styles.formRow}>
            <TextField
              label="Latitude"
              type="number"
              step="0.000001"
              value={form.latitude}
              onChange={(event) => setForm({ ...form, latitude: Number(event.target.value) })}
            />
            <TextField
              label="Longitude"
              type="number"
              step="0.000001"
              value={form.longitude}
              onChange={(event) => setForm({ ...form, longitude: Number(event.target.value) })}
            />
          </div>
          <TextField
            label="Raio (metros)"
            type="number"
            min={20}
            max={200000}
            hint="Entre 20 m e 200 km."
            value={form.radiusMeters}
            onChange={(event) => setForm({ ...form, radiusMeters: Number(event.target.value) })}
          />
        </div>
      </Modal>
    </div>
  );
}
