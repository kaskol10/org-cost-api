import type { PeeringConnectionDetail } from "../types";

function formatCurrency(v: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(v);
}

interface Props {
  peerings: PeeringConnectionDetail[];
}

function DirectionCost({
  title,
  total,
  vpcId,
}: {
  title: string;
  total: number;
  vpcId?: string;
}) {
  return (
    <div className="peering-direction">
      <div className="peering-direction-head">
        <strong>{title}</strong>
        <span>{formatCurrency(total)}</span>
      </div>
      {vpcId ? (
        <p className="category-desc" style={{ margin: "0.35rem 0 0" }}>
          Billing VPC: <span className="mono">{vpcId}</span>
        </p>
      ) : (
        <p className="empty" style={{ margin: "0.35rem 0 0" }}>
          Not billed in this account for this direction.
        </p>
      )}
    </div>
  );
}

export default function PeeringDetailCards({ peerings }: Props) {
  if (!peerings.length) {
    return null;
  }

  return (
    <div className="category-section">
      <h3 className="category-subhead">Peering connections — In / Out cost</h3>
      <div className="peering-cards">
        {peerings.map((p) => (
          <article key={p.peering_id} className="peering-card">
            <header className="peering-card-header">
              <div>
                <h4>{p.name}</h4>
                <div className="mono" style={{ fontSize: "0.8rem", color: "var(--muted)" }}>
                  {p.peering_id}
                </div>
              </div>
              <div className="peering-card-meta">
                <span>
                  {p.local_vpc} ↔ {p.peer_vpc}
                </span>
                {p.peer_region && <span>Peer: {p.peer_region}</span>}
                <span className="peering-role">{p.local_role}</span>
              </div>
            </header>
            {p.attribution_note && (
              <p className="category-desc">{p.attribution_note}</p>
            )}
            <div className="peering-directions">
              <DirectionCost
                title="Out (sending VPC)"
                total={p.out_total}
                vpcId={p.out_vpc}
              />
              <DirectionCost
                title="In (receiving VPC)"
                total={p.in_total}
                vpcId={p.in_vpc}
              />
            </div>
          </article>
        ))}
      </div>
    </div>
  );
}
