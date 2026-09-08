interface PeriodProgressBarProps {
  start?: string | null;
  end?: string | null;
  // Optional label override; defaults to the percentage.
  label?: string;
  // Reference timestamp; defaults to render time. Tests pass this in.
  now?: number;
}

// Shows progress through a date range. Falls back gracefully when only one
// endpoint is known. Past-end-date renders as 100% (overdue/expired).
export function PeriodProgressBar({ start, end, label, now: nowProp }: PeriodProgressBarProps) {
  if (!start && !end) {
    return null;
  }

  // eslint-disable-next-line react-hooks/purity -- visual indicator depends on current time
  const now = nowProp ?? Date.now();
  const startMs = start ? new Date(start).getTime() : null;
  const endMs = end ? new Date(end).getTime() : null;

  let percent = 0;
  if (startMs != null && endMs != null && endMs > startMs) {
    percent = Math.max(0, Math.min(100, ((now - startMs) / (endMs - startMs)) * 100));
  } else if (endMs != null && now >= endMs) {
    percent = 100;
  }

  const tone = percent >= 100 ? "overdue" : percent >= 90 ? "warning" : "ok";
  const display = label ?? `${Math.round(percent)}% through period`;

  return (
    <div className="period-bar">
      <div className={`period-bar-fill period-bar-${tone}`} style={{ width: `${percent}%` }} />
      <span className="period-bar-label">{display}</span>
    </div>
  );
}
