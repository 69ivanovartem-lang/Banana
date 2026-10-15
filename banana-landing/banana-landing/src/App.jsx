import { useState } from 'react';
import Header from './components/Header';
import Hero from './components/Hero';
import About from './components/About';
import Auth from './components/Auth';
import Footer from './components/Footer';
import Cloud from './components/Cloud';

function getSession() {
  return localStorage.getItem('banana_session') || sessionStorage.getItem('banana_session');
}

export default function App() {
  const [mode, setMode] = useState('register');
  const [user, setUser] = useState(getSession());

  const scrollToAuth = (m) => {
    setMode(m);
    document.getElementById('auth').scrollIntoView({ behavior: 'smooth' });
  };

  const enterCloud = (nickname) => setUser(nickname);
  const logout = () => {
    localStorage.removeItem('banana_session');
    sessionStorage.removeItem('banana_session');
    setUser(null);
    window.scrollTo(0, 0);
  };

  if (user) {
    return <Cloud user={user} onLogout={logout} />;
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
