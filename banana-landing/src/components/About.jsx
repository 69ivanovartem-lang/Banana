import { useEffect, useRef } from 'react';

const FEATURES = [
  { icon: '💾', title: 'Резервное копирование', text: 'Автоматические бэкапы ваших файлов с версионированием. Никаких потерянных курсовых — всё под защитой.' },
  { icon: '🌐', title: 'Сетевые узлы NAS', text: 'Прямой доступ к сетевым хранилищам колледжа через единый узел. Подключайся из любой аудитории.' },
  { icon: '⚡', title: 'Мгновенная синхронизация', text: 'Файлы обновляются на всех устройствах в реальном времени. Начал на паре — закончил дома.' },
  { icon: '🔒', title: 'Безопасность', text: 'Шифрование при передаче и хранении, разграничение прав доступа для студентов и преподавателей.' },
  { icon: '📁', title: 'Общие папки', text: 'Совместная работа над проектами: общие директории для команд, групп и кружков колледжа.' },
  { icon: '🆓', title: 'Бесплатно для студентов', text: 'Учебный проект — учебные цены: базовые тарифы бесплатны для всех учащихся колледжа.' },
];

export default function About() {
  const ref = useRef(null);

  useEffect(() => {
    const obs = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          entry.target.classList.add('visible');
          obs.unobserve(entry.target);
        }
      },
      { threshold: 0.15 }
    );
    ref.current?.querySelectorAll('.reveal').forEach((n) => obs.observe(n));
    return () => obs.disconnect();
  }, []);

  return (
    <section className="section" id="about" ref={ref}>
      <h2 className="section-title reveal">Что такое <span className="y">BANANA</span>?</h2>
      <p className="section-sub reveal">
        BANANA — это студенческое облако, созданное командой из четырёх энтузиастов.
        Мы объединили резервное копирование и доступ к сетевым узлам хранения
        в один простой сервис для всего колледжа.
      </p>
      <div className="grid" id="features">
        {FEATURES.map((f, i) => (
          <div className="card reveal" key={i} style={{ transitionDelay: `${(i % 3) * 0.08}s` }}>
            <div className="icon">{f.icon}</div>
            <h3>{f.title}</h3>
            <p>{f.text}</p>
          </div>
        ))}
      </div>
      <div className="stats">
        <div className="stat reveal"><div className="num">99.9%</div><div className="lbl">Аптайм сервиса</div></div>
        <div className="stat reveal"><div className="num">10 ГБ</div><div className="lbl">Бесплатно каждому</div></div>
        <div className="stat reveal"><div className="num">256-bit</div><div className="lbl">Шифрование AES</div></div>
        <div className="stat reveal"><div className="num">4</div><div className="lbl">Разработчика в команде</div></div>
      </div>
    </section>
  );
}
