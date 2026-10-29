import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './styles/index.css';
import './styles/theme.css';
import { ThemeProvider } from './context/ThemeProvider';
import App from './app/App';
import { HeroUIProvider, ToastProvider } from '@heroui/react';
import { GoogleOAuthProvider } from '@react-oauth/google';

const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID || '';

const AppTree = (
  <ThemeProvider>
    <HeroUIProvider>
      {/* bottom-center keeps toasts clear of the Ask Penny launcher and side-panel buttons */}
      <ToastProvider placement="bottom-center" toastOffset={16} />
      <App />
    </HeroUIProvider>
  </ThemeProvider>
);
createRoot(document.getElementById('root')!).render(
  <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
    {import.meta.env.RAILWAY_ENVIRONMENT_NAME ? AppTree : <StrictMode>{AppTree}</StrictMode>}
    <StrictMode></StrictMode>,
  </GoogleOAuthProvider>,
);
