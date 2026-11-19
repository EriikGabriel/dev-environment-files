import { animate } from "motion/react";
import { useEffect, useRef } from "react";

/** A number that counts from its previous value to the new one. */
export function CountUp({ value, duration = 0.9, suffix = "" }: { value: number; duration?: number; suffix?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const from = useRef(0);

  useEffect(() => {
    const ctl = animate(from.current, value, {
      duration,
      ease: [0.16, 1, 0.3, 1],
      onUpdate: (v) => {
        if (ref.current) ref.current.textContent = `${Math.round(v)}${suffix}`;
      },
    });
    from.current = value;
    return () => ctl.stop();
  }, [value, duration, suffix]);

  return (
    <span ref={ref} className="count">
      {`${from.current}${suffix}`}
    </span>
  );
}
