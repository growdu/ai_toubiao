# API 接口规范

> 本文档定义 AI 标书自动生成系统的 REST API 接口规范，采用 OpenAPI 3.1 标准。

---

# 一、API 概览

## 1.1 基础信息

| 项目 | 内容 |
|---|---|
| 基础 URL | `/api/v1` |
| 认证 | Bearer Token (JWT) |
| 内容类型 | `application/json` |
| 字符编码 | UTF-8 |

## 1.2 认证

```http
Authorization: Bearer <jwt_token>
```

JWT Claims:
```json
{
  "sub": "user_id",
  "exp": 1735689600,
  "roles": ["user", "admin"]
}
```

## 1.3 通用响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": {},
  "request_id": "req_abc123"
}
```

## 1.4 错误码

| code | message | 说明 |
|---|---|---|
| 0 | success | 成功 |
| 400 | bad_request | 请求参数错误 |
| 401 | unauthorized | 未认证 |
| 403 | forbidden | 无权限 |
| 404 | not_found | 资源不存在 |
| 429 | rate_limited | 请求过于频繁 |
| 500 | internal_error | 服务器内部错误 |

---

# 二、标书管理接口

## 2.1 创建标书

```yaml
POST /api/v1/bids
description: 创建新的标书生成任务
request:
  content_type: multipart/form-data
  fields:
    rfp_file:
      type: file
      required: true
      description: 招标文件 (PDF/Word)
    materials:
      type: file[]
      required: false
      description: 背景材料 (支持多文件)
    config:
      type: json
      description: 标书配置
      properties:
        project_name:
          type: string
          description: 项目名称
        industry:
          type: string
          enum: [it, construction, government, medical, energy, other]
          description: 行业类型
        chapter_granularity:
          type: string
          enum: [coarse, medium, fine]
          default: medium
          description: 章节粒度
        max_concurrency:
          type: integer
          default: 10
          description: 最大并发数
response:
  202:
    description: 任务已创建
    content:
      application/json:
        schema:
          type: object
          properties:
            bid_job_id:
              type: string
              format: uuid
            status:
              type: string
            created_at:
              type: string
              format: date-time
```

## 2.2 获取标书状态

```yaml
GET /api/v1/bids/{bid_job_id}
description: 查询标书生成任务状态
parameters:
  - name: bid_job_id
    in: path
    required: true
    schema:
      type: string
      format: uuid
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            id:
              type: string
              format: uuid
            status:
              type: string
              enum: [pending, planning, awaiting_review_1, writing, auditing, awaiting_review_2, assembling, awaiting_review_3, exporting, completed, failed, paused]
            progress:
              type: object
              properties:
                total_chapters:
                  type: integer
                completed_chapters:
                  type: integer
                total_illustrations:
                  type: integer
                completed_illustrations:
                  type: integer
                audit_issues:
                  type: object
                  properties:
                    critical:
                      type: integer
                    major:
                      type: integer
                    minor:
                      type: integer
            chapters:
              type: array
              items:
                $ref: '#/components/schemas/ChapterSummary'
            created_at:
              type: string
              format: date-time
            updated_at:
              type: string
              format: date-time
            completed_at:
              type: string
              format: date-time
              nullable: true
```

## 2.3 暂停标书

```yaml
POST /api/v1/bids/{bid_job_id}/pause
description: 暂停标书生成任务
response:
  200:
    description: 暂停成功
```

## 2.4 恢复标书

```yaml
POST /api/v1/bids/{bid_job_id}/resume
description: 恢复已暂停的标书生成任务
response:
  200:
    description: 恢复成功
```

## 2.5 列举标书

```yaml
GET /api/v1/bids
description: 列举当前用户的标书列表
parameters:
  - name: status
    in: query
    schema:
      type: string
      enum: [pending, planning, writing, auditing, completed, failed, paused]
  - name: page
    in: query
    schema:
      type: integer
      default: 1
  - name: page_size
    in: query
    schema:
      type: integer
      default: 20
      maximum: 100
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/BidJobSummary'
            total:
              type: integer
            page:
              type: integer
            page_size:
              type: integer
```

---

# 三、章节管理接口

## 3.1 获取章节大纲

```yaml
GET /api/v1/bids/{bid_job_id}/outline
description: 获取章节大纲（POINT-1 确认前）
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            bid_job_id:
              type: string
              format: uuid
            chapters:
              type: array
              items:
                $ref: '#/components/schemas/ChapterSpec'
            scoring_items:
              type: array
              items:
                $ref: '#/components/schemas/ScoringItem'
```

## 3.2 确认章节大纲

```yaml
POST /api/v1/bids/{bid_job_id}/outline/confirm
description: 确认章节大纲，开始章节撰写（POINT-1）
request:
  content_type: application/json
  body:
    type: object
    properties:
      chapters:
        type: array
        items:
          $ref: '#/components/schemas/ChapterSpec'
      confirmed:
        type: boolean
response:
  204:
    description: 确认成功，开始撰写
```

## 3.3 获取章节内容

```yaml
GET /api/v1/bids/{bid_job_id}/chapters/{chapter_id}
description: 获取章节正文内容
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            id:
              type: string
              format: uuid
            bid_job_id:
              type: string
              format: uuid
            title:
              type: string
            level:
              type: integer
            status:
              type: string
            content:
              type: string
              description: Markdown 正文内容
            word_count:
              type: integer
            illustrations:
              type: array
              items:
                $ref: '#/components/schemas/Illustration'
            citations:
              type: array
              items:
                $ref: '#/components/schemas/Evidence'
            version:
              type: integer
            updated_at:
              type: string
              format: date-time
```

## 3.4 更新章节内容

```yaml
PUT /api/v1/bids/{bid_job_id}/chapters/{chapter_id}
description: 用户编辑章节正文
request:
  content_type: application/json
  body:
    type: object
    properties:
      content:
        type: string
        description: Markdown 正文内容
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            content_hash:
              type: string
            version:
              type: integer
            updated_at:
              type: string
              format: date-time
```

## 3.5 重做章节

```yaml
POST /api/v1/bids/{bid_job_id}/chapters/{chapter_id}/redo
description: 重新生成指定章节
response:
  200:
    description: 重做任务已触发
```

---

# 四、图表管理接口

## 4.1 获取图表列表

```yaml
GET /api/v1/bids/{bid_job_id}/illustrations
description: 获取标书的所有图表
parameters:
  - name: chapter_id
    in: query
    schema:
      type: string
      format: uuid
    description: 按章节筛选
  - name: status
    in: query
    schema:
      type: string
      enum: [draft, rendered, failed, replaced]
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/Illustration'
            total:
              type: integer
```

## 4.2 获取图表详情

```yaml
GET /api/v1/bids/{bid_job_id}/illustrations/{illustration_id}
description: 获取指定图表的详细信息
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            id:
              type: string
              format: uuid
            chapter_id:
              type: string
              format: uuid
            type:
              type: string
              enum: [mermaid, ai_image, data_chart, table, smart_crop, formula]
            title:
              type: string
            caption:
              type: string
            source_content:
              type: string
              description: 图表源码/Mermaid/HTML 等
            rendered_path:
              type: string
              nullable: true
            rendered_format:
              type: string
              enum: [png, svg]
              nullable: true
            status:
              type: string
            quality_score:
              type: number
              nullable: true
            fallback_chain:
              type: array
              items:
                $ref: '#/components/schemas/FallbackAttempt'
```

## 4.3 重新生成图表

```yaml
POST /api/v1/bids/{bid_job_id}/illustrations/{illustration_id}/regenerate
description: 重新生成指定图表
request:
  content_type: application/json
  body:
    type: object
    properties:
      engine:
        type: string
        description: 指定渲染引擎（可选）
response:
  200:
    description: 重新生成任务已触发
```

---

# 五、审计接口

## 5.1 获取审计问题列表

```yaml
GET /api/v1/bids/{bid_job_id}/audit-issues
description: 获取所有审计问题（POINT-2）
parameters:
  - name: severity
    in: query
    schema:
      type: string
      enum: [critical, major, minor]
    description: 按严重程度筛选
  - name: dimension
    in: query
    schema:
      type: string
      enum: [basic_info, format, substantive_response, logic, duplicate, dark_label, figure_data, figure_compliance]
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/AuditIssue'
            total:
              type: integer
            by_severity:
              type: object
              properties:
                critical:
                  type: integer
                major:
                  type: integer
                minor:
                  type: integer
```

## 5.2 处理审计问题

```yaml
POST /api/v1/bids/{bid_job_id}/audit-issues/{issue_id}/resolve
description: 处理单个审计问题
request:
  content_type: application/json
  body:
    type: object
    required:
      - action
    properties:
      action:
        type: string
        enum: [auto_fix, manual_edit, ignore]
        description: 处理方式
      payload:
        type: object
        description: 附加数据（如修复后的内容）
response:
  200:
    description: 处理成功
```

## 5.3 确认审计

```yaml
POST /api/v1/bids/{bid_job_id}/confirm-audit
description: 确认审计完成，进入汇总阶段（POINT-2 通过）
response:
  204:
    description: 确认成功
```

---

# 六、导出接口

## 6.1 导出 Word

```yaml
GET /api/v1/bids/{bid_job_id}/export/word
description: 导出 Word 文档（主输出格式）
response:
  200:
    description: Word 文档
    content:
      application/vnd.openxmlformats-officedocument.wordprocessingml.document:
        schema:
          type: string
          format: binary
    headers:
      Content-Disposition:
        description: attachment; filename="标书_{project_name}.docx"
```

## 6.2 导出 PDF

```yaml
GET /api/v1/bids/{bid_job_id}/export/pdf
description: 导出 PDF 文档（衍生格式）
response:
  200:
    description: PDF 文档
    content:
      application/pdf:
        schema:
          type: string
          format: binary
    headers:
      Content-Disposition:
        description: attachment; filename="标书_{project_name}.pdf"
```

## 6.3 导出摘要

```yaml
GET /api/v1/bids/{bid_job_id}/export/summary
description: 导出一页纸摘要
response:
  200:
    description: 摘要文档
    content:
      application/json:
        schema:
          type: object
          properties:
            project_name:
              type: string
            total_chapters:
              type: integer
            total_illustrations:
              type: integer
            coverage_rate:
              type: number
            estimated_score:
              type: number
            key_strengths:
              type: array
              items:
                type: string
            key_risks:
              type: array
              items:
                type: string
```

## 6.4 获取导出清单

```yaml
GET /api/v1/bids/{bid_job_id}/manifest
description: 获取导出产物清单
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            bid_job_id:
              type: string
              format: uuid
            files:
              type: array
              items:
                type: object
                properties:
                  path:
                    type: string
                  size:
                    type: integer
                  checksum:
                    type: string
                  generated_at:
                    type: string
                    format: date-time
```

---

# 七、知识库接口

## 7.1 上传文档

```yaml
POST /api/v1/kb/documents
description: 上传文档到知识库
request:
  content_type: multipart/form-data
  fields:
    file:
      type: file
      required: true
      description: 文档文件 (PDF/Word/Markdown)
    doc_type:
      type: string
      required: true
      enum: [company_profile, historical_bid, tech_doc, case, personnel, qualification, other]
      description: 文档类型
    metadata:
      type: json
      description: 附加元数据
response:
  201:
    description: 上传成功
    content:
      application/json:
        schema:
          type: object
          properties:
            document_id:
              type: string
              format: uuid
            status:
              type: string
```

## 7.2 搜索知识库

```yaml
GET /api/v1/kb/search
description: 搜索知识库
parameters:
  - name: query
    in: query
    required: true
    schema:
      type: string
    description: 搜索关键词
  - name: doc_type
    in: query
    schema:
      type: string
    description: 按文档类型筛选
  - name: top_k
    in: query
    schema:
      type: integer
      default: 10
      maximum: 100
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            items:
              type: array
              items:
                type: object
                properties:
                  document_id:
                    type: string
                    format: uuid
                  content:
                    type: string
                  score:
                    type: number
                  highlights:
                    type: array
                    items:
                      type: string
            total:
              type: integer
```

## 7.3 获取文档列表

```yaml
GET /api/v1/kb/documents
description: 获取知识库文档列表
parameters:
  - name: doc_type
    in: query
    schema:
      type: string
  - name: page
    in: query
    schema:
      type: integer
      default: 1
  - name: page_size
    in: query
    schema:
      type: integer
      default: 20
response:
  200:
    content:
      application/json:
        schema:
          type: object
          properties:
            items:
              type: array
              items:
                $ref: '#/components/schemas/KBDocument'
            total:
              type: integer
```

## 7.4 删除文档

```yaml
DELETE /api/v1/kb/documents/{document_id}
description: 删除知识库文档
response:
  204:
    description: 删除成功
```

---

# 八、数据模型

## 8.1 ChapterSummary

```yaml
ChapterSummary:
  type: object
  properties:
    id:
      type: string
      format: uuid
    title:
      type: string
    level:
      type: integer
    order_index:
      type: integer
    chapter_type:
      type: string
      enum: [cover_summary, project_understanding, technical, business, implementation, team, qualification, service, risk, appendix]
    priority:
      type: string
      enum: [critical, high, normal, low]
    status:
      type: string
    word_count:
      type: integer
      nullable: true
    progress:
      type: number
      description: 完成进度百分比
```

## 8.2 ChapterSpec

```yaml
ChapterSpec:
  type: object
  properties:
    id:
      type: string
      format: uuid
    title:
      type: string
    level:
      type: integer
      minimum: 1
      maximum: 3
    order_index:
      type: integer
    parent_id:
      type: string
      format: uuid
      nullable: true
    chapter_type:
      type: string
    target_word_count:
      type: integer
    min_word_count:
      type: integer
    writing_style:
      type: string
      enum: [formal, concise, technical, narrative]
    priority:
      type: string
      enum: [critical, high, normal, low]
    dependencies:
      type: array
      items:
        type: string
        format: uuid
```

## 8.3 Illustration

```yaml
Illustration:
  type: object
  properties:
    id:
      type: string
      format: uuid
    chapter_id:
      type: string
      format: uuid
    type:
      type: string
      enum: [mermaid, ai_image, data_chart, table, smart_crop, formula]
    title:
      type: string
    caption:
      type: string
    order_in_chapter:
      type: integer
    status:
      type: string
      enum: [draft, rendered, failed, replaced]
    quality_score:
      type: number
      nullable: true
```

## 8.4 ScoringItem

```yaml
ScoringItem:
  type: object
  properties:
    id:
      type: string
    category:
      type: string
      enum: [preliminary_form, preliminary_qual, preliminary_response, detailed_business, detailed_technical]
    weight:
      type: number
    description:
      type: string
    sub_items:
      type: array
      items:
        type: object
        properties:
          id:
            type: string
          description:
            type: string
          weight:
            type: number
```

## 8.5 AuditIssue

```yaml
AuditIssue:
  type: object
  properties:
    id:
      type: string
      format: uuid
    dimension:
      type: string
      enum: [basic_info, format, substantive_response, logic, duplicate, dark_label, figure_data, figure_compliance]
    severity:
      type: string
      enum: [critical, major, minor]
    location:
      type: object
      properties:
        chapter_id:
          type: string
          format: uuid
        paragraph:
          type: string
    issue:
      type: string
    suggestion:
      type: string
    status:
      type: string
      enum: [open, resolved, ignored]
    resolved_at:
      type: string
      format: date-time
      nullable: true
```

## 8.6 Evidence

```yaml
Evidence:
  type: object
  properties:
    id:
      type: string
      format: uuid
    source_type:
      type: string
      enum: [company_profile, historical_bid, tech_doc, case, personnel, qualification]
    source_ref:
      type: string
    content:
      type: string
    reliability_score:
      type: number
```

## 8.7 FallbackAttempt

```yaml
FallbackAttempt:
  type: object
  properties:
    step:
      type: integer
    engine:
      type: string
    status:
      type: string
      enum: [success, failed]
    error:
      type: string
      nullable: true
    duration_ms:
      type: integer
```

## 8.8 KBDocument

```yaml
KBDocument:
  type: object
  properties:
    id:
      type: string
      format: uuid
    name:
      type: string
    doc_type:
      type: string
      enum: [company_profile, historical_bid, tech_doc, case, personnel, qualification, other]
    size:
      type: integer
    uploaded_at:
      type: string
      format: date-time
    status:
      type: string
      enum: [processing, indexed, failed]
```
