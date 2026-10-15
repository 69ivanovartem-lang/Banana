import { useState } from 'react';
import Header from './components/Header';
import Hero from './components/Hero';
import About from './components/About';
import Auth from './components/Auth';
import Footer from './components/Footer';

export default function App() {
  const [mode, setMode] = useState('register');

  const scrollToAuth = (m) => {
    setMode(m);
    document.getElementById('auth').scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <>
      <Header onAuth={scrollToAuth} />
      <Hero onAuth={scrollToAuth} />
      <About />
      <Auth mode={mode} setMode={setMode} />
      <Footer />
    </>
  );
}
