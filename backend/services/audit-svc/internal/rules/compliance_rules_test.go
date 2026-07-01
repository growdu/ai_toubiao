package rules

import "testing"

func TestComplianceRule_ExpiredCertificate(t *testing.T) {
	r := findRule(t, "资质证书过期")
	got := r.Check("C1", "本证书有效期 2020-2023，已过期。")
	if got == "" {
		t.Fatal("expected flag for expired certificate")
	}
	if !contains(got, "已过期") {
		t.Errorf("issue text should mention 已过期, got %q", got)
	}
}

func TestComplianceRule_NoFalsePositive(t *testing.T) {
	r := findRule(t, "资质证书过期")
	if got := r.Check("C1", "本证书有效期 2024-2029，状态正常。"); got != "" {
		t.Errorf("expected no flag for valid cert, got %q", got)
	}
}

func TestComplianceRule_RepeatedRevenueFlag(t *testing.T) {
	r := findRule(t, "财务数据不一致")
	content := "2022 年收入 1 亿\n2023 年收入 1.2 亿\n2024 年收入 1.5 亿\n主营业务收入构成\n其他营收项目"
	got := r.Check("C1", content)
	if got == "" {
		t.Errorf("expected flag for >3 revenue mentions, got none")
	}
}

func TestComplianceRule_DarkBidCaught(t *testing.T) {
	r := findRule(t, "暗标违规")
	cases := []string{
		"我们期望中标，请多关照。",
		"志在必得！",
		"关系到位，价格不是问题。",
	}
	for _, c := range cases {
		if r.Check("C1", c) == "" {
			t.Errorf("expected flag for dark-bid content: %q", c)
		}
	}
	if got := r.Check("C1", "本项目遵循公开公平原则。"); got != "" {
		t.Errorf("expected no flag for clean content, got %q", got)
	}
}

func TestComplianceRule_PersonnelSpam(t *testing.T) {
	r := findRule(t, "人员信息不一致")
	// Personnel chapter with many 工程师 mentions.
	content := "工程师甲 工程师乙 工程师丙 工程师丁 工程师戊 工程师己 工程师庚"
	if r.Check("项目人员", content) == "" {
		t.Error("expected flag for personnel spam")
	}
	// Same content but non-personnel chapter — should NOT trigger.
	if r.Check("技术方案", content) != "" {
		t.Error("personnel rule should not apply outside personnel chapters")
	}
}

func TestAllRules_NonNil(t *testing.T) {
	if len(ComplianceRules) == 0 {
		t.Fatal("ComplianceRules must not be empty")
	}
	for _, r := range ComplianceRules {
		if r.Name == "" {
			t.Error("rule missing Name")
		}
		if r.Check == nil {
			t.Errorf("rule %q missing Check", r.Name)
		}
		if r.Suggestion == "" {
			t.Errorf("rule %q missing Suggestion", r.Name)
		}
	}
}

func findRule(t *testing.T, name string) ComplianceRule {
	t.Helper()
	for _, r := range ComplianceRules {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("rule not found: %s", name)
	return ComplianceRule{}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && stringIndex(s, substr) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
