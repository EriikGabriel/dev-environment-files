// Visual effects that are not tied to a component: the light that follows the
// pointer over cards and the confetti of a finished run.

const reduced = () => window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/**
 * Lets `.card` and `.spot` elements draw a glow under the pointer: it keeps
 * `--mx`/`--my` (the pointer, relative to the element) up to date, and the
 * stylesheet does the rest. One listener for the whole document.
 */
export function installSpotlight() {
  let frame = 0;
  let last: PointerEvent | null = null;
  const paint = () => {
    frame = 0;
    const e = last;
    if (!e) return;
    const el = (e.target as Element | null)?.closest?.<HTMLElement>(".card, .spot");
    if (!el) return;
    const r = el.getBoundingClientRect();
    el.style.setProperty("--mx", `${e.clientX - r.left}px`);
    el.style.setProperty("--my", `${e.clientY - r.top}px`);
  };
  document.addEventListener(
    "pointermove",
    (e) => {
      last = e;
      if (!frame) frame = requestAnimationFrame(paint);
    },
    { passive: true },
  );
}

/** Bursts confetti from the top of the window; the canvas removes itself. */
export function confetti(colors: string[] = ["#5eead4", "#c084fc", "#fbbf24", "#60a5fa", "#f472b6", "#4ade80"]) {
  if (reduced()) return;
  const canvas = document.createElement("canvas");
  canvas.className = "confetti";
  const dpr = window.devicePixelRatio || 1;
  const w = window.innerWidth;
  const h = window.innerHeight;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  document.body.appendChild(canvas);
  const ctx = canvas.getContext("2d");
  if (!ctx) return canvas.remove();
  ctx.scale(dpr, dpr);

  const parts = Array.from({ length: 160 }, (_, i) => {
    // two cannons, one in each upper corner, aiming at the middle
    const left = i % 2 === 0;
    const angle = (left ? -60 : -120) * (Math.PI / 180) + (Math.random() - 0.5) * 0.9;
    const speed = 9 + Math.random() * 9;
    return {
      x: left ? w * 0.1 : w * 0.9,
      y: h * 0.35,
      vx: Math.cos(angle) * speed,
      vy: Math.sin(angle) * speed,
      size: 5 + Math.random() * 6,
      rot: Math.random() * Math.PI,
      spin: (Math.random() - 0.5) * 0.4,
      color: colors[i % colors.length],
      round: Math.random() < 0.3,
    };
  });

  const start = performance.now();
  const tick = (t: number) => {
    const age = t - start;
    ctx.clearRect(0, 0, w, h);
    ctx.globalAlpha = Math.max(0, 1 - Math.max(0, age - 1800) / 900);
    for (const p of parts) {
      p.vy += 0.32;
      p.vx *= 0.985;
      p.vy *= 0.985;
      p.x += p.vx;
      p.y += p.vy;
      p.rot += p.spin;
      ctx.save();
      ctx.translate(p.x, p.y);
      ctx.rotate(p.rot);
      ctx.fillStyle = p.color;
      if (p.round) {
        ctx.beginPath();
        ctx.arc(0, 0, p.size / 2, 0, Math.PI * 2);
        ctx.fill();
      } else {
        ctx.fillRect(-p.size / 2, -p.size / 4, p.size, p.size / 2);
      }
      ctx.restore();
    }
    if (age < 2700) requestAnimationFrame(tick);
    else canvas.remove();
  };
  requestAnimationFrame(tick);
}
