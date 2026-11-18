/** Placeholder blocks with a moving sheen, shown while a tab loads. */
export function Skeleton({ label, rows = 3 }: { label: string; rows?: number }) {
  return (
    <section className="skeleton-page" aria-busy="true" aria-label={label}>
      <div className="skel skel-hero" />
      <div className="skel-grid">
        {Array.from({ length: rows }, (_, i) => (
          <div key={i} className="skel skel-card" style={{ animationDelay: `${i * 0.12}s` }} />
        ))}
      </div>
      <p className="muted skel-label">{label}</p>
    </section>
  );
}
