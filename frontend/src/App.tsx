import { useState } from 'react';
import './App.css';
import { AuthProvider } from '@/state/auth.tsx';
import { useAuth } from '@/hooks/useAuth.ts';
import { Login } from '@/screens/Login/index.tsx';
import { Register } from '@/screens/Register/index.tsx';

type AuthScreen = 'login' | 'register';

function AppContent() {
  const { isAuthenticated, isLoading } = useAuth();
  const [authScreen, setAuthScreen] = useState<AuthScreen>('login');

  if (isLoading) {
    return (
      <div className="loading-screen" aria-busy="true">
        Loading…
      </div>
    );
  }

  if (!isAuthenticated) {
    if (authScreen === 'register') {
      return <Register onNavigateToLogin={() => setAuthScreen('login')} />;
    }
    return <Login onNavigateToRegister={() => setAuthScreen('register')} />;
  }

  // Phase 13 will replace this with the full five-tab navigation shell.
  return <div className="coming-soon">Game coming soon…</div>;
}

export default function App() {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  );
}
