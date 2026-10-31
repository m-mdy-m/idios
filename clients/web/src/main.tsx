import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router';
import App from './app/App';
import './index.css';

if (
  import.meta.env.PROD &&
  'serviceWorker' in navigator &&
  (window.location.protocol === 'https:' || window.location.hostname === 'localhost')
) {
  window.addEventListener('load', () => {
    void navigator.serviceWorker.register('/sw.js').catch(() => {
      // The Web workspace remains usable when service-worker installation is unavailable.
    });
  });
}

const root = document.getElementById('root');
if (!root) throw new Error('IDIOS could not find the application root element.');

createRoot(root).render(<StrictMode><BrowserRouter><App /></BrowserRouter></StrictMode>);
