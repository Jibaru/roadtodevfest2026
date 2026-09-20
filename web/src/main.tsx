import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Link, Route, Routes } from 'react-router-dom';
import { HomePage } from './pages/Home';
import { KaraokePage } from './pages/Karaoke';
import './index.css';

function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-bg text-ink">
      <header className="container-page flex items-center justify-between py-6">
        <Link to="/" className="flex items-baseline gap-2">
          <span className="font-display text-2xl font-semibold tracking-tightest">s1n.go</span>
          <span className="text-xs uppercase tracking-[0.18em] text-mute">parallel karaoke</span>
        </Link>
        <nav className="text-xs uppercase tracking-[0.18em] text-mute">
          <Link to="/" className="hover:text-ink">
            library
          </Link>
        </nav>
      </header>
      <main>{children}</main>
      <footer className="container-page mt-24 py-8 text-xs uppercase tracking-[0.18em] text-mute">
        s1n.go · made with Go + ADK · Google DevFest 2026
      </footer>
    </div>
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/karaoke/:id" element={<KaraokePage />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  </React.StrictMode>,
);
