import { useQuery } from '@tanstack/react-query';

import { geocodingApi } from '@/api/resources';

/** Casa com o arredondamento do backend (~11 m): evita gerar uma chave nova
 * de cache a cada posição de um veículo parado, só por ruído de GPS. */
const PRECISION = 4;

/**
 * Endereço legível para uma coordenada, obtido via geocodificação reversa.
 *
 * O TanStack Query já deduplica e cacheia por chave: duas partes da tela
 * pedindo o mesmo ponto (o mesmo veículo aparecendo em dois cards, por
 * exemplo) disparam uma única requisição.
 */
export function useAddress(lat: number | undefined, lon: number | undefined) {
  const key =
    lat !== undefined && lon !== undefined
      ? `${lat.toFixed(PRECISION)},${lon.toFixed(PRECISION)}`
      : null;

  return useQuery({
    queryKey: ['address', key],
    queryFn: () => geocodingApi.reverse(lat as number, lon as number),
    enabled: key !== null,
    staleTime: 24 * 60 * 60 * 1000,
    gcTime: 24 * 60 * 60 * 1000,
    retry: 1,
    select: (data) => data.address,
  });
}
