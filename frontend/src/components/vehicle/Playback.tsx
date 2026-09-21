import { useEffect, useMemo, useRef, useState } from 'react';

import { formatDateTime, formatSpeed } from '@/services/format';
import type { Position } from '@/types';

import styles from './Playback.module.css';

const SPEEDS = [1, 2, 5, 10] as const;

/** Intervalo base entre quadros na velocidade 1x. */
const FRAME_MS = 700;

interface PlaybackProps {
  positions: Position[];
  onFrame: (position: Position | null) => void;
}

/**
 * Reprodução do trajeto (§26).
 *
 * Avança por índice de ponto, não por tempo real: assim um trecho com poucos
 * registros não trava a reprodução esperando minutos entre um ponto e outro.
 */
export function Playback({ positions, onFrame }: PlaybackProps) {
  const [index, setIndex] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState<(typeof SPEEDS)[number]>(1);

  const timerRef = useRef<number | null>(null);
  const onFrameRef = useRef(onFrame);
  onFrameRef.current = onFrame;

  const total = positions.length;
  const current = total > 0 ? positions[Math.min(index, total - 1)] : null;

  // Trajeto novo reinicia a reprodução.
  useEffect(() => {
    setIndex(0);
    setPlaying(false);
  }, [positions]);

  useEffect(() => {
    onFrameRef.current(current);
  }, [current]);

  useEffect(() => {
    if (!playing || total === 0) return;

    timerRef.current = window.setInterval(() => {
      setIndex((value) => {
        if (value >= total - 1) {
          setPlaying(false);
          return value;
        }
        return value + 1;
      });
    }, FRAME_MS / speed);

    return () => {
      if (timerRef.current) window.clearInterval(timerRef.current);
    };
  }, [playing, speed, total]);

  const progress = useMemo(() => (total > 1 ? (index / (total - 1)) * 100 : 0), [index, total]);

  if (total === 0) return null;

  return (
    <div className={styles.playback}>
      <div className={styles.controls}>
        <button
          type="button"
          className={styles.playButton}
          onClick={() => {
            if (index >= total - 1) setIndex(0);
            setPlaying((value) => !value);
          }}
          aria-label={playing ? 'Pausar reprodução' : 'Reproduzir trajeto'}
        >
          {playing ? '❚❚' : '▶'}
        </button>

        <input
          className={styles.slider}
          type="range"
          min={0}
          max={Math.max(total - 1, 0)}
          value={index}
          onChange={(event) => {
            setPlaying(false);
            setIndex(Number(event.target.value));
          }}
          aria-label="Posição na reprodução"
          aria-valuetext={current ? formatDateTime(current.gpsTimestamp) : undefined}
        />

        <div className={styles.speeds}>
          {SPEEDS.map((option) => (
            <button
              key={option}
              type="button"
              className={`${styles.speedButton} ${option === speed ? styles.speedActive : ''}`}
              onClick={() => setSpeed(option)}
            >
              {option}x
            </button>
          ))}
        </div>
      </div>

      <div className={styles.readout}>
        <span>
          Ponto <span className={styles.readoutValue}>{index + 1}</span> de {total}
        </span>
        {current && (
          <>
            <span>
              Horário{' '}
              <span className={styles.readoutValue}>{formatDateTime(current.gpsTimestamp)}</span>
            </span>
            <span>
              Velocidade <span className={styles.readoutValue}>{formatSpeed(current.speedKmh)}</span>
            </span>
          </>
        )}
        <span>{progress.toFixed(0)}%</span>
      </div>
    </div>
  );
}
