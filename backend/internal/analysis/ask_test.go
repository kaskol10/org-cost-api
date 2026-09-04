package analysis

import "testing"

func TestRouteQuestionOrgSummary(t *testing.T) {
	r := RouteQuestion("What are our top cost drivers?", nil)
	if r.Intent != AskIntentOrgSummary {
		t.Fatalf("got %q, want org_summary", r.Intent)
	}
}

func TestRouteQuestionTrends(t *testing.T) {
	r := RouteQuestion("How did Redshift change vs last month?", nil)
	if r.Intent != AskIntentTrends {
		t.Fatalf("got %q, want trends", r.Intent)
	}
	if r.Service != "Amazon Redshift" {
		t.Fatalf("service %q", r.Service)
	}
}

func TestRouteQuestionSuggestions(t *testing.T) {
	r := RouteQuestion("Where can we save money?", nil)
	if r.Intent != AskIntentSuggestions {
		t.Fatalf("got %q, want suggestions", r.Intent)
	}
}

func TestRouteQuestionWaste(t *testing.T) {
	r := RouteQuestion("Any unattached EBS volumes?", nil)
	if r.Intent != AskIntentWaste {
		t.Fatalf("got %q, want waste", r.Intent)
	}
}

func TestRouteQuestionAccount(t *testing.T) {
	r := RouteQuestion("production account breakdown", []string{"production", "staging"})
	if r.Intent != AskIntentAccount {
		t.Fatalf("got %q, want account", r.Intent)
	}
	if r.Account != "production" {
		t.Fatalf("account %q", r.Account)
	}
}

func TestRouteQuestionAccountNameOnly(t *testing.T) {
	r := RouteQuestion("Tell me about production", []string{"production", "staging"})
	if r.Intent != AskIntentAccount {
		t.Fatalf("got %q, want account", r.Intent)
	}
	if r.Account != "production" {
		t.Fatalf("account %q", r.Account)
	}
}

func TestRouteQuestionNoBackupFalsePositive(t *testing.T) {
	r := RouteQuestion("What's our backup strategy?", nil)
	if r.Intent == AskIntentTrends {
		t.Fatalf("backup should not route to trends, got %q", r.Intent)
	}
}

func TestRouteQuestionNoUpdateFalsePositive(t *testing.T) {
	r := RouteQuestion("update the budget", nil)
	if r.Intent == AskIntentTrends {
		t.Fatalf("update should not route to trends, got %q", r.Intent)
	}
}

func TestContainsWord(t *testing.T) {
	if !ContainsWord("went up last week", "up") {
		t.Fatal("expected whole-word up match")
	}
	if ContainsWord("backup strategy", "up") {
		t.Fatal("up must not match inside backup")
	}
}

func TestRouteQuestionAccountVsTrends(t *testing.T) {
	r := RouteQuestion("How did production change?", []string{"production", "staging"})
	if r.Intent != AskIntentTrends {
		t.Fatalf("got %q, want trends", r.Intent)
	}
}

func TestRouteQuestionSnapshotCostsTrends(t *testing.T) {
	r := RouteQuestion("How did snapshot costs change vs last month?", nil)
	if r.Intent != AskIntentTrends {
		t.Fatalf("got %q, want trends", r.Intent)
	}
}

func TestRouteQuestionSnapshotRetentionNotWaste(t *testing.T) {
	r := RouteQuestion("What's our snapshot retention policy?", nil)
	if r.Intent == AskIntentWaste {
		t.Fatalf("retention policy should not route to waste, got %q", r.Intent)
	}
}

func TestRouteQuestionOldEBSSnapshotsWaste(t *testing.T) {
	r := RouteQuestion("Any old EBS snapshots we can delete?", nil)
	if r.Intent != AskIntentWaste {
		t.Fatalf("got %q, want waste", r.Intent)
	}
}

func TestRouteQuestionFallback(t *testing.T) {
	r := RouteQuestion("hello world", nil)
	if r.Intent != AskIntentFallback {
		t.Fatalf("got %q, want fallback", r.Intent)
	}
}
