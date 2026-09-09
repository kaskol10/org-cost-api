import type { BillingHighlight, HighlightKind } from "../utils/highlights";

const KIND_LABEL: Record<HighlightKind, string> = {
  spend: "Spend",
  trend: "Trend",
  waste: "Waste",
  concentration: "Focus",
  savings: "Savings",
};

interface Props {
  highlights: BillingHighlight[];
  onAskAbout: (question: string) => void;
  compact?: boolean;
}

export default function BillingHighlights({ highlights, onAskAbout, compact }: Props) {
  if (highlights.length === 0) return null;

  return (
    <section
      className={`billing-highlights ${compact ? "billing-highlights-compact" : ""}`}
      aria-label="Billing highlights"
    >
      <div className="billing-highlights-head">
        <h2>{compact ? "Quick highlights" : "What stands out"}</h2>
        <p className="billing-highlights-hint">
          Tap a card or <strong>Ask about this</strong> to start a conversation.
        </p>
      </div>
      <div className="billing-highlights-grid">
        {highlights.map((h) => (
          <article
            key={h.id}
            className={`billing-highlight-card billing-highlight-${h.kind}`}
            role="button"
            tabIndex={0}
            onClick={() => onAskAbout(h.askQuestion)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onAskAbout(h.askQuestion);
              }
            }}
          >
            <span className={`billing-highlight-kind billing-highlight-kind-${h.kind}`}>
              {KIND_LABEL[h.kind]}
            </span>
            <h3 className="billing-highlight-title">{h.title}</h3>
            <p className="billing-highlight-value">{h.value}</p>
            <p className="billing-highlight-detail">{h.detail}</p>
            <button
              type="button"
              className="btn btn-ghost billing-highlight-ask"
              onClick={(e) => {
                e.stopPropagation();
                onAskAbout(h.askQuestion);
              }}
            >
              Ask about this →
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}
