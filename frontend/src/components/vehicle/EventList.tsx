import { EmptyState } from '@/components/ui/EmptyState';
import { eventSeverity, formatDateTime, formatEvent, formatSpeed } from '@/services/format';
import type { VehicleEvent } from '@/types';

import styles from './EventList.module.css';

export function EventList({ events }: { events: VehicleEvent[] }) {
  if (events.length === 0) {
    return <EmptyState icon="🗒" title="Nenhum evento no período" />;
  }

  return (
    <div className={styles.list}>
      {events.map((event) => {
        const severity = eventSeverity(event.type);
        const fenceName = event.metadata?.geofenceName;

        return (
          <div key={event.id} className={styles.item}>
            <span className={`${styles.marker} ${styles[severity]}`} aria-hidden="true" />
            <div className={styles.content}>
              <div className={styles.title}>
                {formatEvent(event.type)}
                {typeof fenceName === 'string' && fenceName ? ` — ${fenceName}` : ''}
              </div>
              <div className={styles.meta}>
                {event.speedKmh !== null && <span>{formatSpeed(event.speedKmh)}</span>}
                {event.latitude !== null && event.longitude !== null && (
                  <span>
                    {event.latitude.toFixed(5)}, {event.longitude.toFixed(5)}
                  </span>
                )}
              </div>
            </div>
            <span className={styles.time}>{formatDateTime(event.timestamp)}</span>
          </div>
        );
      })}
    </div>
  );
}
