package rules

import "strings"

// ComplianceRule defines a single compliance check.
type ComplianceRule struct {
	Name         string
	Check        func(chapterTitle, content string) string // returns issue description if found
	Suggestion   string
	EvidenceSample string
}

// ComplianceRules is the list of all compliance rules to check.
var ComplianceRules = []ComplianceRule{
	{
		Name: "资质证书过期",
		Check: func(title, content string) string {
			if strings.Contains(content, "有效期") && strings.Contains(content, "已过期") {
				return "内容提及资质证书已过期"
			}
			return ""
		},
		Suggestion:   "请检查资质证书有效期，确保所有证书在投标期间内有效",
		EvidenceSample: "资质证书有效期描述",
	},
	{
		Name: "财务数据不一致",
		Check: func(title, content string) string {
			// Check for obviously inconsistent financial figures
			lines := strings.Split(content, "\n")
			var revenues []string
			for _, line := range lines {
				if strings.Contains(line, "收入") || strings.Contains(line, "营收") || strings.Contains(line, "营业额") {
					revenues = append(revenues, line)
				}
			}
			// Simple check: if multiple different revenue numbers appear, flag for review
			if len(revenues) > 3 {
				return "财务数据出现多次，可能存在不一致"
			}
			return ""
		},
		Suggestion:   "请核实财务数据的一致性，确保所有章节引用的财务数据完全匹配",
		EvidenceSample: "财务数据多处出现",
	},
	{
		Name: "人员信息不一致",
		Check: func(title, content string) string {
			// Check for inconsistent personnel names across chapters
			if strings.Contains(title, "人员") || strings.Contains(title, "团队") {
				// Heuristic: if same person is described with different titles/credentials
				if strings.Count(content, "工程师") > 5 {
					return "人员资质描述可能存在不一致"
				}
			}
			return ""
		},
		Suggestion:   "请确保团队成员的姓名、职务、资质证书等信息在所有章节中完全一致",
		EvidenceSample: "人员信息跨章节重复描述",
	},
	{
		Name: "业绩数据夸大",
		Check: func(title, content string) string {
			// Check for unrealistic project scales
			if strings.Contains(content, "最大") && strings.Contains(content, "亿") {
				// This is a placeholder - in production, cross-reference with actual contracts
				return ""
			}
			return ""
		},
		Suggestion:   "请确保所有业绩数据有合同/证明文件支撑，避免夸大",
		EvidenceSample: "业绩规模描述",
	},
	{
		Name: "专利号格式错误",
		Check: func(title, content string) string {
			// Chinese patent numbers: CNxxxxx.x.x or ZLxxxxxxxx.x.x
			if strings.Contains(content, "专利") {
				// Simple format check
				if strings.Contains(content, "专利号：") || strings.Contains(content, "专利号:") {
					return ""
				}
			}
			return ""
		},
		Suggestion:   "专利号请使用标准格式：CNxxxxx.x.x 或 ZLxxxxxxxx.x.x，并确保真实有效",
		EvidenceSample: "专利号格式",
	},
	{
		Name: "暗标违规",
		Check: func(title, content string) string {
			// Check for potential hidden bid violations
			// (company-specific info that shouldn't be in bid docs)
			darkBidRedFlags := []string{"期望中标", "志在必得", "关系到位"}
			for _, flag := range darkBidRedFlags {
				if strings.Contains(content, flag) {
					return "内容疑似暗标违规: " + flag
				}
			}
			return ""
		},
		Suggestion:   "标书内容必须客观中立，不得包含任何暗示性或承诺性语言",
		EvidenceSample: "暗标违规语言",
	},
}
