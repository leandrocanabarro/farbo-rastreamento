import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Navigate, Outlet, Route, Routes } from 'react-router-dom';

import { AppShell } from '@/components/layout/AppShell';
import { Spinner } from '@/components/ui/Spinner';
import { ToastProvider } from '@/components/ui/Toast';
import { RealtimeProvider } from '@/hooks/useRealtime';
import { DashboardPage } from '@/pages/DashboardPage';
import { DevicesPage } from '@/pages/DevicesPage';
import { DiagnosticsPage } from '@/pages/DiagnosticsPage';
import { EventsPage } from '@/pages/EventsPage';
import { GeofencesPage } from '@/pages/GeofencesPage';
import { LoginPage } from '@/pages/LoginPage';
import { VehicleDetailsPage } from '@/pages/VehicleDetailsPage';
import { AuthProvider, useAuth } from '@/stores/AuthContext';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // O WebSocket já mantém os dados frescos; o refetch é rede de segurança.
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 30_000,
    },
  },
});

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <RealtimeProvider>
            <ToastProvider>
              <Routes>
                <Route path="/login" element={<LoginPage />} />

                <Route element={<RequireAuth />}>
                  <Route element={<AppShell />}>
                    <Route index element={<DashboardPage />} />
                    <Route path="veiculos/:id" element={<VehicleDetailsPage />} />
                    <Route path="eventos" element={<EventsPage />} />
                    <Route path="cercas" element={<GeofencesPage />} />

                    <Route element={<RequireAdmin />}>
                      <Route path="dispositivos" element={<DevicesPage />} />
                      <Route path="diagnostico" element={<DiagnosticsPage />} />
                    </Route>
                  </Route>
                </Route>

                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </ToastProvider>
          </RealtimeProvider>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

function RequireAuth() {
  const { user, loading } = useAuth();

  if (loading) return <Spinner label="Carregando" />;
  if (!user) return <Navigate to="/login" replace />;
  return <Outlet />;
}

/**
 * As telas de cadastro e diagnóstico ficam fora do alcance de quem só
 * visualiza. A API repete a verificação: isto aqui é conveniência, não
 * segurança.
 */
function RequireAdmin() {
  const { canManage } = useAuth();
  if (!canManage) return <Navigate to="/" replace />;
  return <Outlet />;
}
