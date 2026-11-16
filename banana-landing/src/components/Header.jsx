export default function Header({ onAuth }) {
  return (
    <header className="header">
      <a className="logo" href="#top">
        <span className="dot">B</span> BANANA
      </a>
      <nav className="nav">
        <a href="#about">О сервисе</a>
        <a href="#features">Возможности</a>
        <a href="#auth">Доступ</a>
      </nav>
      <div className="header-actions">
        <button className="btn btn-ghost" onClick={() => onAuth('login')}>Войти</button>
        <button className="btn btn-accent" onClick={() => onAuth('register')}>Регистрация</button>
      </div>
    </header>
  );
}
