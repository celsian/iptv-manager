package autosearch

import (
	"testing"
)

func TestParseFilterExpressionEmpty(t *testing.T) {
	node, err := ParseFilterExpression("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node != nil {
		t.Error("empty expression should return nil")
	}
	if !MatchFilter(nil, "anything") {
		t.Error("nil filter should match everything")
	}
}

func TestParseFilterExpressionSingleTerm(t *testing.T) {
	node, err := ParseFilterExpression("basketball")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "NCAA Basketball Finals") {
		t.Error("should match 'basketball' in title")
	}
	if MatchFilter(node, "NCAA Football Finals") {
		t.Error("should not match 'basketball' in football title")
	}
}

func TestParseFilterExpressionCaseInsensitive(t *testing.T) {
	node, err := ParseFilterExpression("BASKETBALL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "ncaa basketball") {
		t.Error("matching should be case-insensitive")
	}
}

func TestParseFilterExpressionImplicitAnd(t *testing.T) {
	node, err := ParseFilterExpression("NCAA basketball")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "NCAA Basketball Finals") {
		t.Error("should match when both terms present")
	}
	if MatchFilter(node, "NCAA Football Finals") {
		t.Error("should not match when only one term present")
	}
}

func TestParseFilterExpressionExplicitAnd(t *testing.T) {
	node, err := ParseFilterExpression("NCAA AND basketball")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "NCAA Basketball Finals") {
		t.Error("should match when both terms present")
	}
	if MatchFilter(node, "NCAA Football Finals") {
		t.Error("should not match when only one term present")
	}
}

func TestParseFilterExpressionOr(t *testing.T) {
	node, err := ParseFilterExpression("basketball OR football")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "NCAA Basketball Finals") {
		t.Error("should match basketball")
	}
	if !MatchFilter(node, "NFL Football Game") {
		t.Error("should match football")
	}
	if MatchFilter(node, "NHL Hockey") {
		t.Error("should not match hockey")
	}
}

func TestParseFilterExpressionNot(t *testing.T) {
	node, err := ParseFilterExpression("NCAA !state")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "NCAA Basketball Finals") {
		t.Error("should match NCAA without 'state'")
	}
	if MatchFilter(node, "NCAA State Championship") {
		t.Error("should not match when 'state' is present")
	}
}

func TestParseFilterExpressionNotKeyword(t *testing.T) {
	node, err := ParseFilterExpression("NCAA NOT state")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "NCAA Basketball Finals") {
		t.Error("should match NCAA without 'state'")
	}
	if MatchFilter(node, "NCAA State Championship") {
		t.Error("should not match when 'state' is present")
	}
}

func TestParseFilterExpressionComplex(t *testing.T) {
	node, err := ParseFilterExpression("(NCAA OR BASKETBALL) AND !state")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "NCAA Basketball Finals") {
		t.Error("should match NCAA without state")
	}
	if !MatchFilter(node, "Pro Basketball League") {
		t.Error("should match basketball without state")
	}
	if MatchFilter(node, "NCAA State Championship") {
		t.Error("should not match when state is present")
	}
	if MatchFilter(node, "Hockey Night") {
		t.Error("should not match without NCAA or basketball")
	}
}

func TestParseFilterExpressionNestedParens(t *testing.T) {
	node, err := ParseFilterExpression("NCAA AND (Basketball OR BBall)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "NCAA Basketball Finals") {
		t.Error("should match NCAA + Basketball")
	}
	if !MatchFilter(node, "NCAA BBall Game") {
		t.Error("should match NCAA + BBall")
	}
	if MatchFilter(node, "NCAA Football Game") {
		t.Error("should not match NCAA + Football")
	}
	if MatchFilter(node, "Pro Basketball League") {
		t.Error("should not match Basketball without NCAA")
	}
}

func TestParseFilterExpressionOperatorPrecedence(t *testing.T) {
	// A OR B AND C should be A OR (B AND C)
	node, err := ParseFilterExpression("hockey OR NCAA basketball")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !MatchFilter(node, "Hockey Night") {
		t.Error("should match hockey alone (OR has lower precedence)")
	}
	if !MatchFilter(node, "NCAA Basketball Finals") {
		t.Error("should match NCAA AND basketball")
	}
	if MatchFilter(node, "NCAA Football") {
		t.Error("should not match NCAA without basketball (and no hockey)")
	}
}

func TestParseFilterExpressionInvalid(t *testing.T) {
	_, err := ParseFilterExpression("(NCAA AND")
	if err == nil {
		t.Error("should return error for unclosed parenthesis")
	}
}
