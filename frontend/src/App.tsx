import { useState } from 'react';
import './App.css';
import { AuthProvider } from '@/state/auth.tsx';
import { NavProvider } from '@/state/nav.tsx';
import { useAuth } from '@/hooks/useAuth.ts';
import { useNav } from '@/hooks/useNav.ts';
import { TabBar } from '@/components/TabBar/index.tsx';
import { Login } from '@/screens/Login/index.tsx';
import { Register } from '@/screens/Register/index.tsx';
import { Main } from '@/screens/Main/index.tsx';
import { Shop } from '@/screens/Shop/index.tsx';
import { Towers } from '@/screens/Towers/index.tsx';
import { Alliance } from '@/screens/Alliance/index.tsx';
import { Events } from '@/screens/Events/index.tsx';

type AuthScreen = 'login' | 'register';

function ScreenContent() {
  const { activeTab } = useNav();
  switch (activeTab) {
    case 'main':
      return <Main />;
    case 'shop':
      return <Shop />;
    case 'towers':
      return <Towers />;
    case 'alliance':
      return <Alliance />;
    case 'events':
      return <Events />;
  }
}

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

  return (
    <div className="app-shell">
      <main className="screen-content" role="tabpanel">
        <ScreenContent />
      </main>
      <TabBar />
    </div>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <NavProvider>
        <AppContent />
      </NavProvider>
    </AuthProvider>
  );
}
