import type { LucideIcon } from "lucide-react";

interface SummaryCard {
  label: string;
  value: number;
  icon: LucideIcon;
  variant?: "danger" | "warning" | "info" | "success";
}

interface SummaryCardsProps {
  cards: SummaryCard[];
}

export function SummaryCards({ cards }: SummaryCardsProps) {
  return (
    <div className="summary-cards">
      {cards.map((card) => {
        const Icon = card.icon;
        return (
          <div key={card.label} className="summary-card">
            <div className="summary-card-label">
              <Icon size={14} style={{ marginRight: 6, verticalAlign: "middle" }} />
              {card.label}
            </div>
            <div className="summary-card-value">{card.value}</div>
          </div>
        );
      })}
    </div>
  );
}
