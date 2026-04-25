import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './styles/index.css';
import App from './app/App';
import { HeroUIProvider, ToastProvider } from '@heroui/react';
import { GoogleOAuthProvider } from '@react-oauth/google';

const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID || '';

console.log('GOOGLE_CLIENT_ID', GOOGLE_CLIENT_ID);
createRoot(document.getElementById('root')!).render(
  <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
    <StrictMode>
      <HeroUIProvider>
        <ToastProvider placement="bottom-right" toastOffset={16} />
        <App />
      </HeroUIProvider>
    </StrictMode>
    ,
  </GoogleOAuthProvider>,
);
