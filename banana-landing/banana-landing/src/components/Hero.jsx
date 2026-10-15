export default function Hero({ onAuth }) {
  return (
    <section className="hero" id="top">
      <div className="badge"><span className="pulse"></span> Облачное хранилище нового поколения</div>
      <h1>BANANA</h1>
      <p className="expansion">Backup And Network Attached Node Access</p>
      <p className="tagline">
        Учебное облако колледжа для резервного копирования файлов и доступа к сетевым
        узлам хранения. Быстро, надёжно и с душой — как спелый банан.
      </p>
      <div className="cta">
        <button className="btn btn-accent" onClick={() => onAuth('register')}>Создать аккаунт</button>
        <a className="btn btn-ghost" href="#about">Узнать больше</a>
      </div>
      <div className="scroll-hint">листай вниз</div>
    </section>
  );
}
