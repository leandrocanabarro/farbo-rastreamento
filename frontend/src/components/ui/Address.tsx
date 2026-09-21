import { useAddress } from '@/hooks/useAddress';
import { formatCoordinates } from '@/services/format';

interface AddressProps {
  lat: number;
  lon: number;
  className?: string;
}

/**
 * Mostra o endereço de uma coordenada, com as coordenadas exatas sempre
 * disponíveis no title (passar o mouse) e como reserva enquanto a consulta
 * não volta ou se ela falhar — nunca deixamos a posição sem nenhum texto.
 */
export function Address({ lat, lon, className }: AddressProps) {
  const { data: address, isLoading } = useAddress(lat, lon);
  const coordinates = formatCoordinates(lat, lon);

  return (
    <span className={className} title={coordinates}>
      {address ?? (isLoading ? 'Buscando endereço…' : coordinates)}
    </span>
  );
}
