import { Badge } from '@/components/ui/Badge';
import type { BadgeTone } from '@/components/ui/Badge';
import { EmptyState } from '@/components/ui/EmptyState';
import { formatCommand, formatCommandStatus, formatDateTime } from '@/services/format';
import type { CommandStatus, DeviceCommand } from '@/types';

import styles from './CommandHistory.module.css';

function statusTone(status: CommandStatus): BadgeTone {
  switch (status) {
    case 'ACKNOWLEDGED':
      return 'success';
    case 'SENT':
    case 'SENDING':
    case 'PENDING':
      return 'accent';
    case 'REJECTED':
      return 'warning';
    default:
      return 'danger';
  }
}

export function CommandHistory({ commands }: { commands: DeviceCommand[] }) {
  if (commands.length === 0) {
    return <EmptyState icon="⌘" title="Nenhum comando enviado ainda" />;
  }

  return (
    <div className={styles.list}>
      {commands.map((command) => (
        <div key={command.id} className={styles.item}>
          <div className={styles.header}>
            <span className={styles.name}>{formatCommand(command.command)}</span>
            <Badge tone={statusTone(command.status as CommandStatus)}>
              {formatCommandStatus(command.status)}
            </Badge>
          </div>

          <div className={styles.meta}>
            <span>Pedido: {formatDateTime(command.requestedAt)}</span>
            {command.acknowledgedAt && <span>Resposta: {formatDateTime(command.acknowledgedAt)}</span>}
          </div>

          {command.payload && <div className={styles.payload}>{command.payload}</div>}
          {command.response && <div className={styles.message}>{command.response}</div>}
          {command.error && <div className={styles.message}>{command.error}</div>}
        </div>
      ))}
    </div>
  );
}
