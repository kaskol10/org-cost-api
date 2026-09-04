"""Regression tests for account cost JSON shaping."""

from org_cost_mcp.account_costs import shape_account_costs


def test_shape_account_costs_other_services_total():
    acct = {
        "account_id": "123456789012",
        "account_name": "production",
        "costs": {
            "start": "2026-07-11",
            "end": "2026-08-10",
            "all_total": 5000.0,
            "total": 800.0,
            "other_services_total": 4200.0,
            "by_service": [{"service": "Amazon S3", "amount": 3000.0}],
            "usage_categories": [
                {"id": "ebs", "label": "EBS volumes", "amount": 400.0},
            ],
        },
    }
    out = shape_account_costs(acct)
    assert out["other_services_usd"] == 4200.0
    assert out["ec2_other_usd"] == 800.0
    assert out["all_services_total_usd"] == 5000.0
    assert out["account_name"] == "production"
    assert len(out["top_services"]) == 1
    assert out["usage_categories"][0]["id"] == "ebs"


def test_shape_account_costs_api_period():
    data = {
        "account_id": "123456789012",
        "account_name": "production",
        "period": {"start": "2026-01-01", "end": "2026-01-31"},
        "costs": {"all_total": 100.0},
    }
    out = shape_account_costs(data)
    assert out["period"] == {"start": "2026-01-01", "end": "2026-01-31"}


def test_shape_account_costs_missing_fields():
    out = shape_account_costs({"account_id": "1", "account_name": "dev"})
    assert out["other_services_usd"] is None
    assert out["top_services"] == []
    assert out["usage_categories"] == []
