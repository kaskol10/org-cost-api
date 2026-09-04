import { useState } from "react";
import EBSSnapshotPanel from "./EBSSnapshotPanel";
import EBSVolumePanel from "./EBSVolumePanel";
import PeeringDetailCards from "./PeeringDetailCards";
import type { UsageCategory } from "../types";
import { formatGiB } from "../utils/format";

function formatCurrency(v: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(v);
}

interface Props {
  categories: UsageCategory[];
  total: number;
}

export default function UsageCategoryPanel({ categories, total }: Props) {
  const [openId, setOpenId] = useState<string | null>(
    categories[0]?.id ?? null
  );

  if (!categories.length) {
    return (
      <div>
        <h2>Cost by category</h2>
        <p className="empty">No usage type breakdown for this period.</p>
      </div>
    );
  }

  return (
    <div>
      <h2>Cost by category</h2>
      <p className="category-hint">
        Expand a category for details. EBS volumes and snapshots show live count,
        total size, and daily cost. VPC peering shows estimated In/Out per connection.
      </p>
      <ul className="category-list">
        {categories.map((cat) => {
          const open = openId === cat.id;
          const isPeering = cat.id === "vpc-peering";
          const isEBSVolumes = cat.id === "ebs-volumes";
          const isEBSSnapshots = cat.id === "ebs-snapshots";
          const inventoryBadge =
            cat.inventory_count != null && cat.inventory_count > 0
              ? `${cat.inventory_count.toLocaleString()} · ${formatGiB(cat.inventory_size_gib ?? 0)}`
              : null;
          const showOps = (cat.api_operations?.length ?? 0) > 0;
          const peeringDetails = cat.peering_details ?? [];
          const hasUsageTypes = (cat.usage_types?.length ?? 0) > 0;

          return (
            <li key={cat.id} className={`category-item ${open ? "open" : ""}`}>
              <button
                type="button"
                className="category-header"
                onClick={() => setOpenId(open ? null : cat.id)}
              >
                <span className="category-bar-wrap">
                  <span
                    className="category-bar"
                    style={{ width: `${Math.min(cat.percent, 100)}%` }}
                  />
                </span>
                <span className="category-meta">
                  <span>
                    <strong>{cat.label}</strong>
                    {inventoryBadge && (
                      <span className="category-inventory mono">{inventoryBadge}</span>
                    )}
                  </span>
                  <span className="category-amount">
                    {formatCurrency(cat.amount)}
                    <span className="category-pct">
                      {cat.percent.toFixed(1)}%
                    </span>
                  </span>
                </span>
              </button>
              {open && (
                <div className="category-detail">
                  <p className="category-desc">{cat.description}</p>

                  {isEBSVolumes && cat.ebs_volume_detail && (
                    <EBSVolumePanel detail={cat.ebs_volume_detail} />
                  )}

                  {isEBSSnapshots && cat.ebs_snapshot_detail && (
                    <EBSSnapshotPanel detail={cat.ebs_snapshot_detail} />
                  )}

                  {isPeering && (
                    <>
                      {cat.peering_drivers_note && (
                        <p className="category-desc">{cat.peering_drivers_note}</p>
                      )}
                      {peeringDetails.length > 0 ? (
                        <PeeringDetailCards peerings={peeringDetails} />
                      ) : (
                        <p className="empty">
                          No VPC peering connections found in this account/region,
                          or peering costs may appear under{" "}
                          <span className="mono">Data transfer</span> as legacy{" "}
                          <span className="mono">DataTransfer-Regional-Bytes</span>.
                        </p>
                      )}
                    </>
                  )}

                  {showOps && !isEBSVolumes && !isEBSSnapshots && (
                    <div className="category-section">
                      <h3 className="category-subhead">By API operation</h3>
                      <p className="category-desc">
                        {isPeering
                          ? "Traffic direction in billing (VpcPeering-In/Out-Bytes)."
                          : "Costs broken down by EC2 API operation."}
                      </p>
                      <div className="table-wrap">
                        <table>
                          <thead>
                            <tr>
                              <th>API operation</th>
                              <th className="amount">Of category</th>
                              <th className="amount">Cost</th>
                            </tr>
                          </thead>
                          <tbody>
                            {cat.api_operations!.map((op) => (
                              <tr key={op.operation}>
                                <td className="mono">{op.operation}</td>
                                <td className="amount">
                                  {op.percent != null
                                    ? `${op.percent.toFixed(1)}%`
                                    : "—"}
                                </td>
                                <td className="amount">
                                  {formatCurrency(op.amount)}
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  )}

                  {hasUsageTypes ? (
                    <div className="category-section">
                      <h3 className="category-subhead">By usage type (region)</h3>
                      <div className="table-wrap">
                        <table>
                          <thead>
                            <tr>
                              <th>Region</th>
                              <th>Usage type</th>
                              <th className="amount">Share</th>
                              <th className="amount">Cost</th>
                            </tr>
                          </thead>
                          <tbody>
                            {cat.usage_types.map((u) => (
                              <tr key={u.usage_type}>
                                <td className="mono">{u.region || "—"}</td>
                                <td className="mono">
                                  {u.short_name || u.usage_type}
                                </td>
                                <td className="amount">
                                  {total > 0
                                    ? `${((u.amount / total) * 100).toFixed(1)}%`
                                    : "—"}
                                </td>
                                <td className="amount">
                                  {formatCurrency(u.amount)}
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  ) : (
                    <p className="empty">No usage type lines for this category.</p>
                  )}
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
