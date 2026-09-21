import L from 'leaflet';

/**
 * Ícones do mapa.
 *
 * O marcador é um SVG inline em vez de imagem: assim ele acompanha os tokens
 * de cor do tema e gira conforme o rumo sem precisar de sprite por ângulo.
 */

/** Verde = ignição ligada, vermelho = desligada, cinza = ainda sem leitura. */
const IGNITION_ON = '#3fbf7f';
const IGNITION_OFF = '#e2574c';
const IGNITION_UNKNOWN = '#6b7f95';

/** Cor do selo de bloqueio, deliberadamente distinta do vermelho de ignição
 * desligada: as duas coisas podem ocorrer juntas e precisam ser distinguíveis
 * à primeira vista (ver docs sobre ACC × relé serem sinais independentes). */
const BLOCKED_BADGE = '#b3261e';

/** Altura reservada ao selo de bloqueio, na mesma unidade do desenho do
 * veículo (que usa um viewBox fixo de 32 unidades de largura). */
const BADGE_BAND = 14;

interface VehicleIconOptions {
  /**
   * Estado da ignição (ACC), lido do próprio rastreador.
   * true = ligada (verde) · false = desligada (vermelho) · null = sem leitura
   * ainda (cinza).
   */
  ignition: boolean | null;
  heading: number | null;
  /** Veículo parado ganha um círculo; em movimento, uma seta. */
  moving: boolean;
  selected: boolean;
  /**
   * Relé de corte acionado. Mostra um selo de cadeado acima do marcador — não
   * muda a cor do marcador em si, porque ignição e bloqueio são dois sinais
   * independentes do rastreador (o motor pode estar bloqueado com a ignição
   * ligada ou desligada).
   */
  blocked: boolean;
  /**
   * Dispositivo sem comunicação recente (STALE/OFFLINE): o ícone fica
   * esmaecido para avisar que a cor pode não refletir o estado atual.
   */
  online?: boolean;
}

export function vehicleIcon({
  ignition,
  heading,
  moving,
  selected,
  blocked,
  online = true,
}: VehicleIconOptions): L.DivIcon {
  const color = ignition === null ? IGNITION_UNKNOWN : ignition ? IGNITION_ON : IGNITION_OFF;
  const rotation = heading ?? 0;
  const size = selected ? 38 : 32;

  // O veículo é sempre desenhado no mesmo espaço de 32x32 unidades; quando há
  // selo de bloqueio, o viewBox cresce para cima e o grupo do veículo desce
  // pelo tamanho da faixa reservada — o desenho do veículo em si não muda.
  const bandUnits = blocked ? BADGE_BAND : 0;
  const totalUnits = 32 + bandUnits;
  const scale = size / 32;
  const pixelWidth = size;
  const pixelHeight = scale * totalUnits;

  const ring = selected
    ? `<circle cx="16" cy="16" r="15" fill="none" stroke="${color}" stroke-width="1.5" opacity="0.5"/>`
    : '';

  const shape = moving
    ? `<path d="M16 5 L23 25 L16 20.5 L9 25 Z" fill="${color}" stroke="#0b1017" stroke-width="1.5" stroke-linejoin="round" transform="rotate(${rotation} 16 16)"/>`
    : `<circle cx="16" cy="16" r="7.5" fill="${color}" stroke="#0b1017" stroke-width="2"/>`;

  const badge = blocked ? blockedBadge(16, BADGE_BAND / 2) : '';

  const html = `
    <svg width="${pixelWidth}" height="${pixelHeight}" viewBox="0 0 32 ${totalUnits}"
         xmlns="http://www.w3.org/2000/svg" ${online ? '' : 'opacity="0.55"'}>
      ${badge}
      <g transform="translate(0, ${bandUnits})">${ring}${shape}</g>
    </svg>`;

  // O ponto de ancoragem fica sempre no centro do veículo (nunca no selo),
  // para o marcador continuar exatamente sobre a coordenada do GPS.
  const anchorY = scale * (16 + bandUnits);

  return L.divIcon({
    className: 'vehicle-marker',
    html,
    iconSize: [pixelWidth, pixelHeight],
    iconAnchor: [pixelWidth / 2, anchorY],
    // Abre acima de tudo o que está desenhado no topo do ícone — o selo,
    // quando existe, ou o próprio veículo quando não há bloqueio.
    popupAnchor: [0, -anchorY],
  });
}

/** Selo de motor bloqueado: halo + círculo + cadeado, para chamar atenção
 * mesmo num ícone pequeno no mapa. */
function blockedBadge(cx: number, cy: number): string {
  return `
    <circle cx="${cx}" cy="${cy}" r="8" fill="${BLOCKED_BADGE}" opacity="0.25"/>
    <circle cx="${cx}" cy="${cy}" r="6" fill="${BLOCKED_BADGE}" stroke="#ffffff" stroke-width="1.25"/>
    <text x="${cx}" y="${cy + 2.5}" font-size="7" text-anchor="middle">🔒</text>`;
}

/** Marcador pequeno para os pontos do histórico. */
export function historyIcon(color = '#3d9ae8'): L.DivIcon {
  return L.divIcon({
    className: 'history-marker',
    html: `<svg width="10" height="10" viewBox="0 0 10 10"><circle cx="5" cy="5" r="3.5" fill="${color}" stroke="#0b1017" stroke-width="1.5"/></svg>`,
    iconSize: [10, 10],
    iconAnchor: [5, 5],
  });
}

/** Bandeiras de início e fim de um trajeto. */
export function endpointIcon(kind: 'start' | 'end'): L.DivIcon {
  const color = kind === 'start' ? '#3fbf7f' : '#e2574c';
  return L.divIcon({
    className: 'endpoint-marker',
    html: `<svg width="18" height="18" viewBox="0 0 18 18"><circle cx="9" cy="9" r="7" fill="${color}" stroke="#0b1017" stroke-width="2"/></svg>`,
    iconSize: [18, 18],
    iconAnchor: [9, 9],
  });
}
