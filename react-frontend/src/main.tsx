import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/index.css'
import App from './app/App';
import { HeroUIProvider, ToastProvider } from '@heroui/react';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <HeroUIProvider>
      <ToastProvider placement="bottom-right" toastOffset={16} />
      <App />
    </HeroUIProvider>
  </StrictMode>,
)
