package config

import "testing"

type testRule struct {
	Name string `json:"name"`
	Skip bool   `json:"skip_retry_on_failure,omitempty"`
}

type testConfig struct {
	Rules []testRule `json:"rules"`
}

func TestUpdateConfigFromMapResetsOmittedFieldsInSliceElements(t *testing.T) {
	cfg := &testConfig{
		Rules: []testRule{
			{Name: "default-a", Skip: true},
			{Name: "default-b", Skip: true},
		},
	}

	err := UpdateConfigFromMap(cfg, map[string]string{
		"rules": `[{"name":"rule-a"},{"name":"rule-b"}]`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap returned error: %v", err)
	}

	for i, rule := range cfg.Rules {
		if rule.Skip {
			t.Fatalf("rule %d retained stale skip value after JSON update: %+v", i, rule)
		}
	}
}
