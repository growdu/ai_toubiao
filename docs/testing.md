# 测试策略与用例

> 本文档定义 AI 标书自动生成系统的测试策略、测试用例和验收标准。

---

# 一、测试策略

## 1.1 测试金字塔

```
                    ┌─────────────┐
                    │    E2E      │  ← 端到端验收测试
                    │   Tests    │     (少量，高价值)
                   ─┴─────────────┴─
                  ┌─────────────────┐
                  │  Integration   │  ← 服务集成测试
                  │    Tests       │    (中量)
                 ─┴─────────────────┴─
                ┌───────────────────────┐
                │      Unit Tests       │  ← 单元测试
                │      (大量)           │    (核心逻辑)
               ─┴───────────────────────┴─
```

| 层级 | 占比 | 目标 | 测试范围 |
|---|---|---|---|
| 单元测试 | 70% | 快速反馈，覆盖核心逻辑 | 服务函数、工具类、业务规则 |
| 集成测试 | 20% | 验证组件协作 | API 端点、数据库操作、队列消息 |
| E2E 测试 | 10% | 验证完整用户流程 | 端到端标书生成流程 |

## 1.2 测试环境

| 环境 | 用途 | 数据库 | LLM |
|---|---|---|---|
| **local** | 开发自测 | PostgreSQL (本地) | Mock |
| **dev** | 开发联调 | PostgreSQL | 真实 API (限流) |
| **staging** | 预发布验证 | PostgreSQL (生产镜像) | 真实 API |
| **prod** | 生产验证 | PostgreSQL | 真实 API |

## 1.3 测试技术栈

| 类型 | 工具 | 框架 |
|---|---|---|
| Go 单元测试 | `testing` 包 | testify |
| Go 集成测试 | `testcontainers-go` | - |
| API E2E | `playwright` / `httptest` | - |
| 前端单元 | Vitest | React Testing Library |
| 前端 E2E | Playwright | - |
| 性能测试 | k6 | - |
| Mock LLM | [un官嘲] 模拟服务 | - |

---

# 二、单元测试用例

## 2.1 RFP 解析模块

### test_rfp_parser_basic

```go
func TestRFPParser_BasicPDF(t *testing.T) {
    // Given: 一份标准 PDF 招标文件
    pdfFile := "testdata/sample_rfp.pdf"

    // When: 调用解析服务
    result, err := rfpp.Parse(context.Background(), pdfFile)

    // Then: 解析结果符合预期
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, "项目名称", result.Metadata.ProjectName)
    assert.Equal(t, "IT信息化建设", result.Industry)
    assert.True(t, len(result.Sections) > 0)
    assert.True(t, len(result.StarClauses) > 0)
}

func TestRFPParser_ScannedPDF(t *testing.T) {
    // Given: 扫描件 PDF
    pdfFile := "testdata/scanned_rfp.pdf"

    // When: 调用解析服务
    result, err := rfpp.Parse(context.Background(), pdfFile)

    // Then: OCR 成功解析
    assert.NoError(t, err)
    assert.True(t, result.Metadata.HasOCR)
    assert.True(t, len(result.Sections) > 0)
}
```

### test_rfp_parser_star_clauses

```go
func TestRFPParser_StarClausesDetection(t *testing.T) {
    tests := []struct {
        name       string
        content    string
        expectStar bool
    }{
        {
            name:       "★号标记条款",
            content:    "★投标人必须具备 ISO9001 认证",
            expectStar: true,
        },
        {
            name:       "普通条款",
            content:    "投标人应具有良好的商业信誉",
            expectStar: false,
        },
        {
            name:       "废标条款",
            content:    "不满足上述条件的视为废标",
            expectStar: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, _ := rfpp.ParseScoringItems(tt.content)
            assert.Equal(t, tt.expectStar, len(result.StarClauses) > 0)
        })
    }
}
```

## 2.2 章节规划模块

### test_chapter_planner_granularity

```go
func TestChapterPlanner_AdaptiveGranularity(t *testing.T) {
    tests := []struct {
        name              string
        rfpWordCount      int
        expectedMinChapters int
        expectedMaxChapters int
    }{
        {
            name:              "小规模 RFP (5万字)",
            rfpWordCount:      50000,
            expectedMinChapters: 20,
            expectedMaxChapters: 40,
        },
        {
            name:              "中规模 RFP (10万字)",
            rfpWordCount:      100000,
            expectedMinChapters: 35,
            expectedMaxChapters: 55,
        },
        {
            name:              "大规模 RFP (20万字)",
            rfpWordCount:      200000,
            expectedMinChapters: 60,
            expectedMaxChapters: 100,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            specs, _ := planner.PlanChapters(tt.rfpWordCount)
            assert.True(t, len(specs) >= tt.expectedMinChapters)
            assert.True(t, len(specs) <= tt.expectedMaxChapters)
        })
    }
}

func TestChapterPlanner_PriorityAssignment(t *testing.T) {
    // Given: 含合规条款的 RFP
    rfp := &ParsedRFP{
        Sections: []RMSection{
            {Title: "商务标", HasCompliance: true},
            {Title: "技术方案", Weight: 40},
            {Title: "资质要求", HasCompliance: true, Weight: 20},
        },
    }

    // When: 生成章节规划
    specs, _ := planner.PlanChapters(rfp)

    // Then: 合规条款章节优先级为 critical
    for _, spec := range specs {
        if spec.HasCompliance {
            assert.Equal(t, PriorityCritical, spec.Priority)
        }
    }
}
```

### test_chapter_planner_dependency

```go
func TestChapterPlanner_DependencyAnalysis(t *testing.T) {
    // Given: 章节依赖关系
    specs := []*ChapterSpec{
        {ID: "ch1", Title: "封面与目录"},
        {ID: "ch2", Title: "技术方案"},
        {ID: "ch3", Title: "技术方案详情"},  // 依赖 ch2
        {ID: "ch4", Title: "资质证明"},
    }

    // When: 分析依赖
    graph := planner.BuildDependencyGraph(specs)

    // Then: ch3 依赖 ch2
    assert.Contains(t, graph["ch2"], "ch3")
    assert.NotContains(t, graph["ch1"], "ch3")

    // Then: 无环
    assert.NoError(t, planner.DetectCycle(graph))
}
```

## 2.3 章节撰写模块

### test_chapter_writer_content

```go
func TestChapterWriter_MinWordCount(t *testing.T) {
    // Given: 章节规格
    spec := &ChapterSpec{
        ID:              "ch1",
        MinWordCount:    800,
        WritingStyle:    "formal",
        ChapterType:     "technical",
    }

    // When: 撰写章节
    result, err := writer.WriteChapter(context.Background(), spec, nil)

    // Then: 字数满足要求
    assert.NoError(t, err)
    assert.True(t, result.WordCount >= spec.MinWordCount)
}

func TestChapterWriter_TermConsistency(t *testing.T) {
    // Given: 全局术语表
    glossary := &Glossary{
        Terms: map[string]string{
            "API":  "应用程序编程接口",
            "SDK":  "软件开发工具包",
        },
    }

    spec := &ChapterSpec{
        ID:         "ch1",
        Glossary:   glossary,
        ChapterType: "technical",
    }

    // When: 撰写章节
    result, err := writer.WriteChapter(context.Background(), spec, nil)

    // Then: 术语一致性
    assert.NoError(t, err)
    // 章节内容应使用标准化术语
}
```

### test_chapter_writer_figure_placeholder

```go
func TestChapterWriter_FigurePlaceholderExtraction(t *testing.T) {
    // Given: 章节正文含图表占位符
    content := `
    系统采用分层架构，如 [!figure:arch-overview type=mermaid caption=系统分层架构图] 所示。

    数据流向如 [!figure:data-flow type=mermaid caption=数据流程图] 所示。
    `

    // When: 提取占位符
    specs := writer.ExtractFigureSpecs(content)

    // Then: 正确识别 2 个图表
    assert.Len(t, specs, 2)
    assert.Equal(t, "arch-overview", specs[0].ID)
    assert.Equal(t, "mermaid", specs[0].Type)
    assert.Equal(t, "data-flow", specs[1].ID)
}
```

## 2.4 图表渲染模块

### test_illustrator_mermaid

```go
func TestIllustrator_RenderMermaid(t *testing.T) {
    // Given: 有效的 Mermaid 源码
    source := `
    flowchart TD
        A[用户] --> B[API]
        B --> C[数据库]
    `

    // When: 渲染
    result, err := illustrator.RenderMermaid(context.Background(), source)

    // Then: 成功渲染
    assert.NoError(t, err)
    assert.Equal(t, "rendered", result.Status)
    assert.NotNil(t, result.RenderedPath)
    assert.True(t, result.QualityScore > 0.8)
}

func TestIllustrator_MermaidFallback(t *testing.T) {
    // Given: 无效的 Mermaid 源码
    source := `flowchart TD invalid syntax`

    // When: 渲染（触发降级）
    result, err := illustrator.RenderMermaid(context.Background(), source)

    // Then: 降级到本地渲染
    assert.NoError(t, err)
    assert.Len(t, result.FallbackChain) > 0
}
```

### test_illustrator_datachart

```go
func TestIllustrator_RenderDataChart(t *testing.T) {
    // Given: 有效数据
    data := &DataSeries{
        Name: "性能指标",
        X:    []string{"Q1", "Q2", "Q3", "Q4"},
        Y:    [][]float64{{100, 150, 120, 180}},
    }

    // When: 渲染数据图表
    result, err := illustrator.RenderDataChart(context.Background(), data, "bar")

    // Then: 成功渲染
    assert.NoError(t, err)
    assert.Equal(t, "rendered", result.Status)
    assert.Equal(t, "bar", result.ChartType)
}
```

## 2.5 审计模块

### test_auditor_basic_info

```go
func TestAuditor_BasicInfoConsistency(t *testing.T) {
    // Given: 标书内容
    content := &BidContent{
        BidLetter:  "投标总报价：人民币壹佰万元整（¥1,000,000）",
        PriceTable: "报价：1,000,000 元",
    }

    // When: 审计
    issues := auditor.AuditBasicInfo(content)

    // Then: 发现金额不一致
    assert.True(t, hasIssue(issues, "price_inconsistency"))
}

func TestAuditor_CertExpiry(t *testing.T) {
    // Given: 含过期证书的标书
    content := &BidContent{
        Qualifications: []Qualification{
            {Name: "ISO9001", ExpiryDate: time.Now().AddDate(0, -1, 0)}, // 已过期
        },
        BidDeadline: time.Now().AddDate(0, 1, 0),
    }

    // When: 审计
    issues := auditor.AuditBasicInfo(content)

    // Then: 发现证书过期
    assert.True(t, hasIssue(issues, "cert_expired_before_deadline"))
}
```

### test_auditor_response_completeness

```go
func TestAuditor_StarClauseResponse(t *testing.T) {
    // Given: ★号条款和响应
    starClause := &StarClause{
        Text:     "投标人必须具备 ISO9001 认证",
        Severity: "rejection",
    }
    content := &BidContent{
        StarClauses:    []*StarClause{starClause},
        ResponseMatrix: []*ResponseItem{},  // 未响应
    }

    // When: 审计实质性响应
    issues := auditor.AuditSubstantiveResponse(content)

    // Then: 发现 ★号条款未响应
    assert.True(t, hasIssue(issues, "star_clause_unresponded"))
    assert.Equal(t, "critical", getIssue(issues, "star_clause_unresponded").Severity)
}
```

## 2.6 状态机测试

### test_bid_job_state_machine

```sql
-- 状态转换测试用例
| 当前状态 | 事件 | 期望状态 |
|---|---|---|
| pending | start | planning |
| planning | outline_confirmed | writing |
| writing | all_chapters_done | auditing |
| auditing | audit_passed | assembling |
| auditing | audit_failed | awaiting_review_2 |
| assembling | assembled | awaiting_review_3 |
| awaiting_review_3 | confirmed | exporting |
| exporting | exported | completed |
```

---

# 三、集成测试用例

## 3.1 API 集成测试

### test_api_create_bid

```go
func TestAPI_CreateBid(t *testing.T) {
    // Given: 已认证的用户和 RFP 文件
    setupTestUser(t)
    rfpFile := mustCreateTempFile(t, "rfp.pdf", sampleRFPContent)

    // When: 调用创建标书 API
    resp := httptest.NewRecorder()
    req, _ := http.NewRequest("POST", "/api/v1/bids", multipartBody(rfpFile))
    req.Header.Set("Authorization", "Bearer "+testToken)
    router.ServeHTTP(resp, req)

    // Then: 返回 202 和 job_id
    assert.Equal(t, 202, resp.Code)
    var result map[string]interface{}
    json.Unmarshal(resp.Body.Bytes(), &result)
    assert.NotEmpty(t, result["bid_job_id"])
}
```

### test_api_chapter_lifecycle

```go
func TestAPI_ChapterLifecycle(t *testing.T) {
    // Given: 已创建的标书
    bid := createTestBid(t)

    // When: 获取章节列表
    resp := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/v1/bids/"+bid.ID+"/chapters", nil)
    router.ServeHTTP(resp, req)

    // Then: 返回章节列表
    assert.Equal(t, 200, resp.Code)
    var chapters []ChapterSummary
    json.Unmarshal(resp.Body.Bytes(), &chapters)
    assert.True(t, len(chapters) > 0)

    // When: 更新章节内容
    updateReq := map[string]string{"content": "# Updated content\n\nNew content here."}
    body, _ := json.Marshal(updateReq)
    req, _ = http.NewRequest("PUT", "/api/v1/bids/"+bid.ID+"/chapters/"+chapters[0].ID, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp = httptest.NewRecorder()
    router.ServeHTTP(resp, req)

    // Then: 更新成功
    assert.Equal(t, 200, resp.Code)
}
```

## 3.2 数据库集成测试

### test_db_chapter_content_versioning

```go
func TestDB_ChapterContentVersioning(t *testing.T) {
    // Given: 章节内容
    specID := createTestChapterSpec(t)

    // When: 保存第一个版本
    v1, err := contents.Save(specID, "Version 1 content", 1)
    assert.NoError(t, err)

    // When: 保存第二个版本
    v2, err := contents.Save(specID, "Version 2 content with more detail", 2)
    assert.NoError(t, err)

    // Then: 可以获取历史版本
    getV1, err := contents.GetByVersion(specID, 1)
    assert.NoError(t, err)
    assert.Equal(t, "Version 1 content", getV1.Content)

    getV2, err := contents.GetByVersion(specID, 2)
    assert.NoError(t, err)
    assert.Equal(t, "Version 2 content with more detail", getV2.Content)

    // Then: 最新版本是 v2
    latest, err := contents.GetLatest(specID)
    assert.NoError(t, err)
    assert.Equal(t, 2, latest.Version)
}
```

## 3.3 队列集成测试

### test_queue_chapter_pipeline

```go
func TestQueue_ChapterPipeline(t *testing.T) {
    // Given: 章节任务
    spec := createTestChapterSpec(t)

    // When: 入队章节任务
    err := chapterPipeline.Enqueue(spec.ID)
    assert.NoError(t, err)

    // Then: 任务进入队列
    queueLen, err := asynqClient.QueueLength("chapter-q")
    assert.NoError(t, err)
    assert.True(t, queueLen > 0)

    // When: Worker 处理任务
    handler.ProcessNextTask()

    // Then: 任务完成
    spec = reloadChapterSpec(spec.ID)
    assert.Equal(t, "done", spec.Status)
}
```

---

# 四、E2E 测试用例

## 4.1 完整标书生成流程

### test_e2e_complete_bid_generation

```go
func TestE2E_CompleteBidGeneration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E test in short mode")
    }

    // Given: 完整的 RFP 和材料
    user := loginTestUser(t)
    rfpPath := uploadRFP(t, user, "testdata/complete_rfp.pdf")

    // When: 创建标书任务
    bid := createBid(t, user, rfpPath)

    // Then: 等待规划完成
    bid = waitForStatus(t, bid.ID, "awaiting_review_1", 5*time.Minute)

    // When: 确认大纲
    confirmOutline(t, bid.ID)

    // Then: 等待撰写完成
    bid = waitForStatus(t, bid.ID, "auditing", 15*time.Minute)

    // When: 确认审计
    confirmAudit(t, bid.ID)

    // Then: 等待汇总完成
    bid = waitForStatus(t, bid.ID, "completed", 5*time.Minute)

    // When: 导出 Word
    wordDoc := exportWord(t, bid.ID)

    // Then: Word 文档有效
    assert.True(t, len(wordDoc) > 0)
    assert.True(t, isValidDocx(wordDoc))
}
```

## 4.2 图表生成与插入流程

### test_e2e_illustration_generation

```go
func TestE2E_IllustrationGeneration(t *testing.T) {
    // Given: 包含图表需求的章节
    bid := createBidWithIllustrations(t)

    // When: 等待图表生成
    illustrations := waitForIllustrationsRendered(t, bid.ID, 3, 5*time.Minute)

    // Then: 图表渲染成功
    for _, ill := range illustrations {
        assert.Equal(t, "rendered", ill.Status)
        assert.NotEmpty(t, ill.RenderedPath)
    }

    // When: 导出 Word
    wordDoc := exportWord(t, bid.ID)

    // Then: Word 包含图表
    assert.True(t, wordDocContainsImages(wordDoc))
}
```

---

# 五、性能测试

## 5.1 性能测试用例

### test_perf_rfp_parsing

```go
func BenchmarkRfpParsing_1MB(b *testing.B) {
    // Given: 1MB RFP 文件
    file := generateTestRFP(1 << 20) // 1MB

    // When: 解析
    for i := 0; i < b.N; i++ {
        rfpp.Parse(context.Background(), file)
    }
}
// 目标: < 10s for 1MB RFP
```

### test_perf_chapter_writing

```go
func BenchmarkChapterWriting(b *testing.B) {
    // Given: 标准章节规格
    spec := &ChapterSpec{
        TargetWordCount: 2000,
        ChapterType:     "technical",
        WritingStyle:    "formal",
    }

    // When: 撰写
    for i := 0; i < b.N; i++ {
        writer.WriteChapter(context.Background(), spec, nil)
    }
}
// 目标: < 30s per chapter
```

### test_perf_illustration_rendering

```go
func BenchmarkMermaidRendering(b *testing.B) {
    // Given: 标准 Mermaid 图
    source := generateStandardArchitectureDiagram()

    // When: 渲染
    for i := 0; i < b.N; i++ {
        illustrator.RenderMermaid(context.Background(), source)
    }
}
// 目标: < 5s per mermaid diagram
```

## 5.2 性能指标

| 指标 | 目标 | 告警阈值 |
|---|---|---|
| RFP 解析 (1MB) | < 10s | > 15s |
| 章节撰写 (2000字) | < 30s | > 45s |
| 图表渲染 (Mermaid) | < 5s | > 10s |
| 图表渲染 (DALL-E) | < 15s | > 30s |
| 100 章节并发 | < 5min | > 8min |
| API P99 延迟 | < 500ms | > 1s |
| 数据库查询 P99 | < 100ms | > 200ms |

---

# 六、验收测试清单

## 6.1 功能验收

| ID | 功能 | 验收条件 |
|---|---|---|
| AC-1 | RFP 解析 | 能正确解析 PDF/Word 格式招标文件，提取章节结构、评分项、★号条款 |
| AC-2 | 章节规划 | 自动生成章节大纲，支持用户调整章节粒度和顺序 |
| AC-3 | 章节撰写 | 生成的章节内容字数达标、术语一致、符合写作风格 |
| AC-4 | 图表生成 | 支持 Mermaid、表格、数据图表，渲染成功率高 |
| AC-5 | 图表插入 | 图表正确插入到正文对应位置，编号连续 |
| AC-6 | 审计功能 | 能发现金额不一致、证书过期、★号条款未响应等问题 |
| AC-7 | 审计分级 | 审计问题按 critical/major/minor 分级 |
| AC-8 | Word 导出 | 导出的 Word 文档格式正确，包含目录、章节、图表 |
| AC-9 | 状态暂停/恢复 | 标书可暂停，暂停后可恢复继续 |
| AC-10 | 章节重做 | 支持单个章节重新生成 |

## 6.2 性能验收

| ID | 指标 | 验收条件 |
|---|---|---|
| PC-1 | RFP 解析 | 100 万字 RFP 解析 < 60s |
| PC-2 | 标书生成 | 50 章节标书生成 < 10min |
| PC-3 | 图表生成 | 单张标准图表 < 30s |
| PC-4 | 系统可用性 | ≥ 99.5% |

## 6.3 安全验收

| ID | 指标 | 验收条件 |
|---|---|---|
| SC-1 | 认证 | 未认证用户无法访问 API |
| SC-2 | 授权 | 用户只能访问自己的标书 |
| SC-3 | 敏感信息 | 暗标合规检测能识别暴露企业信息的内容 |
