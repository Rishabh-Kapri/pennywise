import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { GoogleOAuthProvider } from '@react-oauth/google';
import './styles/index.css';
import './styles/theme.css';
import { config } from './config/env';
import { ThemeProvider } from './components/theme/ThemeProvider';
import App from './app/App';
import ToastHost from './components/common/ToastHost/ToastHost';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <GoogleOAuthProvider clientId={config.googleClientId}>
      <ThemeProvider>
        <ToastHost />
        <App />
      </ThemeProvider>
    </GoogleOAuthProvider>
  </StrictMode>,
);
