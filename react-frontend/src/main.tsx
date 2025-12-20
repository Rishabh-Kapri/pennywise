import { Profiler, StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/index.css'
import App from './app/App';

function onRender(id: string, phase: string, actualDuration: number) {
  console.log(id, phase, actualDuration)
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
      <App />
  </StrictMode>,
)
