import type { ReactNode } from "react";

interface PriorityIndicatorProps {
  priority: string;
  children: ReactNode;
}

export function PriorityIndicator({ priority, children }: PriorityIndicatorProps) {
  return (
    <div className={`priority-indicator priority-${priority}`}>
      {children}
    </div>
  );
}
