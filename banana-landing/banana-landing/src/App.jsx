import { useState, useEffect } from 'react';
import Header from './components/Header';
import Hero from './components/Hero';
import About from './components/About';
import Auth from './components/Auth';
import Footer from './components/Footer';
import Cloud from './components/Cloud';
import { currentUser, logout as apiLogout, getToken } from './api';

export default function App() {
  const [mode, setMode] = useState('register');
  // сессия = email + живой токен на стороне бэкенда
  const [user, setUser] = useState(() => (getToken() ? currentUser() : null));

  // если токен протух (401 в любом запросе) — выкидываем на лендинг
  useEffect(() => {
    const onUnauthorized = () => setUser(null);
    window.addEventListener('banana:unauthorized', onUnauthorized);
    return () => window.removeEventListener('banana:unauthorized', onUnauthorized);
  }, []);

  const scrollToAuth = (m) => {
    setMode(m);
    document.getElementById('auth').scrollIntoView({ behavior: 'smooth' });
  };

  const enterCloud = (email) => setUser(email);
  const handleLogout = async () => {
    await apiLogout(); // гасим сессию на сервере
    setUser(null);
    window.scrollTo(0, 0);
  };

  if (user) {
    return <Cloud user={user} onLogout={handleLogout} />;
  }

  return (
    <>
      <Header onAuth={scrollToAuth} />
      <Hero onAuth={scrollToAuth} />
      <About />
      <Auth mode={mode} setMode={setMode} onSuccess={enterCloud} />
      <Footer />
    </>
  );
}
