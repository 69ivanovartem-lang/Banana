import { useState } from 'react';
import { register, login, setSession } from '../api';

const MIN_PASSWORD = 6; // столько же требует Go-бэкенд (/api/v1/auth/register)

export default function Auth({ mode, setMode, onSuccess }) {
  const [form, setForm] = useState({ email: '', password: '', remember: true });
  const [msg, setMsg] = useState(null);
  const [busy, setBusy] = useState(false);
  const isRegister = mode === 'register';

  async function submit(e) {
    e.preventDefault();
    const email = form.email.trim();
    const password = form.password;

    if (!email || !password) {
      setMsg({ type: 'err', text: 'Заполните все поля' });
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setMsg({ type: 'err', text: 'Введите корректный email' });
      return;
    }
    if (password.length < MIN_PASSWORD) {
      setMsg({ type: 'err', text: `Пароль должен быть не короче ${MIN_PASSWORD} символов` });
      return;
    }

    setBusy(true);
    setMsg(null);
    try {
      if (isRegister) {
        await register(email, password);
        // после регистрации сразу логинимся — бэкенд выдаёт токен сессии
        const s = await login(email, password);
        setSession(s.token, email, form.remember);
        if (onSuccess) onSuccess(email);
      } else {
        const s = await login(email, password);
        setSession(s.token, email, form.remember);
        setMsg({ type: 'ok', text: 'С возвращением! 🍌' });
        if (onSuccess) setTimeout(() => onSuccess(email), 350);
      }
    } catch (err) {
      let text = err.message || 'Не удалось выполнить запрос';
      if (isRegister && err.status === 409) text = 'Такой email уже зарегистрирован — войдите';
      if (!isRegister && err.status === 401) text = 'Неверный email или пароль';
      setMsg({ type: 'err', text });
    } finally {
      setBusy(false);
    }
  }

  const switchMode = (m) => (e) => {
    e.preventDefault();
    setMode(m);
    setMsg(null);
  };

  return (
    <section className="section" id="auth">
      <h2 className="section-title" style={{ textAlign: 'center' }}>Начни прямо <span className="y">сейчас</span></h2>
      <p className="section-sub" style={{ textAlign: 'center', margin: '0 auto 48px' }}>
        Зарегистрируйся по email и получи 10 ГБ облачного пространства бесплатно.
      </p>
      <div className="auth-wrap">
        <div className="tabs">
          <button className={'tab' + (isRegister ? ' active' : '')} onClick={switchMode('register')}>Регистрация</button>
          <button className={'tab' + (!isRegister ? ' active' : '')} onClick={switchMode('login')}>Вход</button>
        </div>
        <form onSubmit={submit}>
          <div className="field">
            <label>Email</label>
            <input type="email" placeholder="you@example.com" value={form.email} autoComplete="email"
              onChange={(e) => setForm({ ...form, email: e.target.value })} />
          </div>
          <div className="field">
            <label>Пароль (минимум {MIN_PASSWORD} символов)</label>
            <input type="password" placeholder="••••••••" value={form.password}
              autoComplete={isRegister ? 'new-password' : 'current-password'}
              onChange={(e) => setForm({ ...form, password: e.target.value })} />
          </div>
          <label className="field" style={{ display: 'flex', gap: 8, alignItems: 'center', cursor: 'pointer' }}>
            <input type="checkbox" checked={form.remember} style={{ width: 'auto' }}
              onChange={(e) => setForm({ ...form, remember: e.target.checked })} />
            <span>Запомнить меня</span>
          </label>
          <button className="btn btn-accent btn-block" type="submit" disabled={busy}>
            {busy ? 'Подождите…' : isRegister ? 'Создать аккаунт' : 'Войти'}
          </button>
        </form>
        {msg && <div className={'auth-msg ' + msg.type}>{msg.text}</div>}
        <p className="form-note">
          {isRegister
            ? <>Уже есть аккаунт? <a href="#" onClick={switchMode('login')}>Войти</a></>
            : <>Нет аккаунта? <a href="#" onClick={switchMode('register')}>Зарегистрироваться</a></>}
        </p>
      </div>
    </section>
  );
}
