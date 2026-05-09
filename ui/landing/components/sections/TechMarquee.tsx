const ITEMS = [
  "LANGCHAIN",
  "PROMETHEUS",
  "FASTAPI",
  "NEXT.JS",
  "QDRANT",
  "NEO4J",
  "MONGODB",
  "LOKI",
  "GRAFANA",
  "ALERTMANAGER",
  "MEM0AI",
  "PYTHON 3.13",
];

export function TechMarquee() {
  const sequence = [...ITEMS, ...ITEMS];

  return (
    <section
      aria-label="Technology stack ticker"
      className="relative overflow-hidden py-5"
      style={{
        background: "#111111",
        borderTop: "3px solid #111111",
        borderBottom: "3px solid #111111",
      }}
    >
      <div className="flex animate-marquee whitespace-nowrap">
        {sequence.map((item, i) => (
          <span
            key={`${item}-${i}`}
            className="font-pixel text-[#FFE600] text-xs uppercase tracking-wider px-6 flex items-center gap-6"
          >
            {item}
            <span className="text-[#FF3131]">★</span>
          </span>
        ))}
      </div>
    </section>
  );
}

export default TechMarquee;
