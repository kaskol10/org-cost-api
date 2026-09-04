import Markdown from "./Markdown";
import type { SuggestionsResponse } from "../types";
import { formatCurrency } from "../utils/format";
import {
  CATEGORY_META,
  groupSuggestionsByCategory,
  normalizeCategory,
  summarizeSuggestions,
} from "../utils/suggestions";

interface Props {
  suggestions: SuggestionsResponse | null;
  loading?: boolean;
  enriching?: boolean;
  onAskAbout?: (question: string) => void;
}

function suggestionAskQuestion(title: string): string {
  return `Tell me more about this savings opportunity: "${title}". What should we do first and what's the impact?`;
}

function SavingsBadge({ amount }: { amount: number | undefined }) {
  if (amount != null && amount > 0) {
    return (
      <span className="suggestion-savings-badge">
        ~{formatCurrency(amount)}
        <span className="suggestion-savings-period">/mo</span>
      </span>
    );
  }
  return <span className="suggestion-savings-badge suggestion-savings-investigate">Investigate</span>;
}

export default function SuggestionsPanel({
  suggestions,
  loading,
  enriching,
  onAskAbout,
}: Props) {
  if (loading) {
    return (
      <section className="panel suggestions-panel">
        <h2>Cost savings opportunities</h2>
        <p className="loading">Loading suggestions…</p>
      </section>
    );
  }

  if (!suggestions || suggestions.suggestions.length === 0) {
    return null;
  }

  const summary = summarizeSuggestions(suggestions.suggestions);
  const groups = groupSuggestionsByCategory(suggestions.suggestions);

  return (
    <section className="panel suggestions-panel" aria-label="Cost savings opportunities">
      <div className="suggestions-header">
        <div>
          <h2>Cost savings opportunities</h2>
          {suggestions.period && (
            <p className="suggestions-period">Period: {suggestions.period}</p>
          )}
        </div>
        <div className="suggestions-header-badges">
          {suggestions.llm_enriched && (
            <span className="suggestions-ai-badge">AI explained</span>
          )}
          {enriching && !suggestions.llm_enriched && (
            <span className="suggestions-ai-badge suggestions-ai-badge-pending">
              Adding AI insights…
            </span>
          )}
        </div>
      </div>

      <div className="suggestions-summary-bar" role="region" aria-label="Savings summary">
        <div className="suggestions-kpi suggestions-kpi-primary">
          <span className="suggestions-kpi-label">Quantified savings</span>
          <span className="suggestions-kpi-value">
            {summary.quantifiedCount > 0 ? (
              <>
                ~{formatCurrency(summary.totalQuantifiedSavingsUsd)}
                <span className="suggestions-kpi-sub">/month</span>
              </>
            ) : (
              <span className="suggestions-kpi-muted">—</span>
            )}
          </span>
          <span className="suggestions-kpi-hint">
            {summary.quantifiedCount > 0
              ? `${summary.quantifiedCount} of ${summary.totalCount} items with $ estimates`
              : "No dollar estimates — review trends & waste below"}
          </span>
        </div>
        <div className="suggestions-kpi">
          <span className="suggestions-kpi-label">Opportunities</span>
          <span className="suggestions-kpi-value">{summary.totalCount}</span>
        </div>
        <div className="suggestions-category-chips" aria-label="By category">
          {summary.byCategory.map((cat) => (
            <div
              key={cat.category}
              className={`suggestions-category-chip ${cat.cssClass}`}
              title={cat.hint}
            >
              <span className="suggestions-category-chip-label">{cat.label}</span>
              <span className="suggestions-category-chip-count">{cat.count}</span>
              {cat.quantifiedSavingsUsd > 0 && (
                <span className="suggestions-category-chip-savings">
                  ~{formatCurrency(cat.quantifiedSavingsUsd)}/mo
                </span>
              )}
            </div>
          ))}
        </div>
      </div>

      {(suggestions.narrative_summary || suggestions.summary) && (
        <p className="suggestions-overview">
          {suggestions.narrative_summary ?? suggestions.summary}
        </p>
      )}

      <div className="suggestions-groups">
        {groups.map(({ category, items }) => (
          <section
            key={category.category}
            className={`suggestions-group ${category.cssClass}`}
            aria-labelledby={`suggestions-group-${category.category}`}
          >
            <header className="suggestions-group-head">
              <h3 id={`suggestions-group-${category.category}`}>
                <span className={`suggestion-category-pill ${category.cssClass}`}>
                  {category.label}
                </span>
                <span className="suggestions-group-meta">
                  {items.length} {items.length === 1 ? "item" : "items"}
                  {category.quantifiedSavingsUsd > 0 && (
                    <> · ~{formatCurrency(category.quantifiedSavingsUsd)}/mo</>
                  )}
                </span>
              </h3>
              <p className="suggestions-group-hint">{category.hint}</p>
            </header>

            <ul className="suggestions-list">
              {items.map((item) => {
                const cat = normalizeCategory(item.category);
                const meta = CATEGORY_META[cat];
                return (
                  <li
                    key={item.id}
                    className={`suggestion-card ${meta.cssClass}`}
                  >
                    <div className="suggestion-card-top">
                      <div className="suggestion-card-title-row">
                        <span className="suggestion-priority" title="Priority rank">
                          #{item.priority}
                        </span>
                        <span className={`suggestion-category-pill ${meta.cssClass}`}>
                          {meta.label}
                        </span>
                        {item.confidence && (
                          <span className="suggestion-confidence">{item.confidence}</span>
                        )}
                      </div>
                      <SavingsBadge amount={item.estimated_monthly_usd} />
                    </div>

                    <h4 className="suggestion-title">{item.title}</h4>

                    {(item.account || item.service) && (
                      <div className="suggestion-tags">
                        {item.account && (
                          <span className="suggestion-tag">Account: {item.account}</span>
                        )}
                        {item.service && (
                          <span className="suggestion-tag">Service: {item.service}</span>
                        )}
                      </div>
                    )}

                    <div className="suggestion-breakdown">
                      <div className="suggestion-breakdown-row">
                        <span className="suggestion-breakdown-label">What we see</span>
                        <p>{item.detail}</p>
                      </div>

                      {item.explanation && (
                        <div className="suggestion-breakdown-row suggestion-explanation">
                          <span className="suggestion-breakdown-label">Why it matters</span>
                          <Markdown className="md-content">{item.explanation}</Markdown>
                        </div>
                      )}

                      {item.actions.length > 0 && (
                        <div className="suggestion-breakdown-row">
                          <span className="suggestion-breakdown-label">Next steps</span>
                          <ul className="suggestion-actions">
                            {item.actions.map((action) => (
                              <li key={action}>{action}</li>
                            ))}
                          </ul>
                        </div>
                      )}

                      {onAskAbout && (
                        <button
                          type="button"
                          className="btn btn-ghost suggestion-ask-btn"
                          onClick={() => onAskAbout(suggestionAskQuestion(item.title))}
                        >
                          Know more about this →
                        </button>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          </section>
        ))}
      </div>

      {suggestions.additional_insights && suggestions.additional_insights.length > 0 && (
        <div className="suggestions-extra">
          <h3>Additional insights</h3>
          <ul>
            {suggestions.additional_insights.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        </div>
      )}

      {!suggestions.llm_enriched && !enriching && (
        <p className="suggestions-footnote">
          Rule-based opportunities. Set <code>VITE_CHAT_URL</code> + LLM for richer “Why it
          matters” explanations.
        </p>
      )}
    </section>
  );
}
