#!/usr/bin/env python3
"""Generate ground-truth OpenAPI 3.0.3 spec from Go code.

Outputs:
  - docs/api/openapi.generated.yaml        (single, self-contained)
  - docs/api/services/<svc>.yaml           (per-service path slices)
"""
import pathlib, re

ROOT = pathlib.Path('.')
OUT_FULL = ROOT / 'docs/api/openapi.generated.yaml'
OUT_SVC  = ROOT / 'docs/api/services'

METHOD_CALL_RE = re.compile(r'\b\.(Get|Post|Put|Patch|Delete|Head|Options)\(\s*"([^"]+)"\s*,\s*([A-Za-z_][\w.]*)')
GROUP_RE = re.compile(r'\bRoute\(\s*"([^"]+)"\s*,\s*func\s*\(')

GW_PREFIX = {
    'project-svc':   '/api/v1/projects',
    'document-svc':  '/api/v1/documents',
    'workflow-svc':  '/api/v1/bids',
    'knowledge-svc': '/api/v1/kb',
    'audit-svc':     '/api/v1/audit',
    'template-svc':  '/api/v1/templates',
    'billing-svc':   '/api/v1/billing',
    'notify-svc':    '/api/v1/notifications',
    'router-svc':    '/api/v1/router',
    'doc-gen':       '/api/v1/docgen',
}
SVC_DESC = {
    'project-svc':   '项目 / 标段 CRUD',
    'document-svc':  '招标与素材文档的上传 / 解析 / Markdown 化',
    'workflow-svc':  'Step01-05 工作流编排、状态机、章节生成与导出',
    'knowledge-svc': '知识库素材、pgvector 检索、chunk 管理',
    'audit-svc':     '四层一致性审计 + agent 自动修复',
    'template-svc':  '行业模板 / 模板市集',
    'billing-svc':   '订阅、Token 用量、硬限保护',
    'notify-svc':    '邮件 / Webhook / IM 推送与用户偏好',
    'router-svc':    '多模型路由、缓存、用量统计',
    'doc-gen':       '独立 docx / pdf 导出引擎',
}
TAG = {svc: svc.replace('-svc', '') for svc in GW_PREFIX}
TAG['doc-gen'] = 'docgen'
TAG['router-svc'] = 'router'

def parse_routes(text):
    """Yield (method, internal_full_path, handler, lineno) from chi-style registration."""
    lines = text.splitlines()
    stack, depth = [], 0
    for i, line in enumerate(lines, 1):
        gm = GROUP_RE.search(line)
        if gm:
            stack.append((gm.group(1), depth))
            depth += line.count('{') - line.count('}')
            continue
        for mm in METHOD_CALL_RE.finditer(line):
            method, path, handler = mm.group(1).upper(), mm.group(2), mm.group(3)
            if path in ('/healthz', '/readyz') or not path.startswith('/'):
                continue
            full = ''.join(p for p, _ in stack) + path
            yield (method, full, handler, i)
        depth += line.count('{') - line.count('}')
        stack = [(p, d) for p, d in stack if d < depth]

def collect_routes():
    """Return [(svc, method, public_path, handler, file, line)]."""
    out = []
    for p in sorted((ROOT / 'services').rglob('internal/api/*.go')):
        if p.name.endswith('_test.go'):
            continue
        if 'Routes' not in p.read_text(encoding='utf-8', errors='ignore'):
            continue
        svc = p.parts[1]
        text = p.read_text(encoding='utf-8', errors='ignore')
        rel = str(p.relative_to(ROOT / 'services' / svc))
        for m, pp, h, ln in parse_routes(text):
            gp = GW_PREFIX.get(svc)
            if not gp:
                continue
            parts = pp.split('/')
            if len(parts) >= 4 and parts[1] == 'api' and parts[2] == 'v1':
                tail = '/'.join(parts[4:])
                pub = gp + ('/' + tail if tail else '')
            else:
                pub = gp + pp
            pub = pub.rstrip('/') or gp
            out.append((svc, m, pub, h, rel, ln))
    # auth routes registered directly in api-gateway main.go
    out.append(('api-gateway', 'POST', '/api/v1/auth/login',    'loginHandler',    'cmd/api-gateway/main.go', 85))
    out.append(('api-gateway', 'POST', '/api/v1/auth/register', 'registerHandler', 'cmd/api-gateway/main.go', 86))
    out.append(('api-gateway', 'POST', '/api/v1/auth/refresh',  'refreshHandler',  'cmd/api-gateway/main.go', 87))
    # dedup by (svc,method,path)
    seen, uniq = set(), []
    for r in out:
        k = (r[0], r[1], r[2])
        if k in seen: continue
        seen.add(k); uniq.append(r)
    return uniq

def path_params(p):
    out = []
    for name in re.findall(r'\{([^{}]+)\}', p):
        out.append({'name': name, 'in': 'path', 'required': True,
                    'schema': {'type': 'string', 'format': 'uuid'},
                    'description': f'Path parameter `{name}`.'})
    return out

def handler_to_summary(h):
    h = h.rsplit('.', 1)[-1]
    return re.sub(r'([a-z])([A-Z])', r'\1 \2', h).strip() or h

def yaml_escape(s):
    s = s.replace('\\', '\\\\').replace('"', '\\"')
    return f'"{s}"'

def slug(s):
    return re.sub(r'\W+', '_', s).strip('_').lower()

def build_op(method, path, svc, handler, file, ln, *, public=True):
    tag = TAG.get(svc, svc)
    op = {
        'summary': handler_to_summary(handler),
        'operationId': f'{slug(svc)}_{method.lower()}_{slug(path)}',
        'tags': [tag],
        'parameters': path_params(path),
        'responses': {
            '200': {'description': 'OK',
                    'content': {'application/json': {'schema': {'$ref': '#/components/schemas/OKEnvelope'}}}},
            '400': {'$ref': '#/components/responses/BadRequest'},
            '401': {'$ref': '#/components/responses/Unauthorized'},
            '404': {'$ref': '#/components/responses/NotFound'},
            '500': {'$ref': '#/components/responses/Internal'},
        },
        'x-handler': f'{svc}.{handler}',
        'x-source': f'{file}:{ln}',
        'x-stub': 'needs-swag-annotation',
    }
    if not public:
        op['security'] = []
    else:
        op['security'] = [{'bearerAuth': []}]
    if method in ('POST', 'PUT', 'PATCH'):
        op['requestBody'] = {'required': True, 'content': {'application/json': {'schema': {'$ref': '#/components/schemas/GenericRequest'}}}}
    return op

def render_paths_block(paths_by_method, indent=2):
    """Render a `paths:` entry block where each line is indented `indent` spaces."""
    spc = ' ' * indent
    sub = ' ' * (indent + 4)
    out = []
    for p in sorted(paths_by_method.keys()):
        out.append(f"{spc}{p}:")
        for method in ('get', 'post', 'put', 'patch', 'delete', 'head', 'options'):
            if method not in paths_by_method[p]:
                continue
            op = paths_by_method[p][method]
            out.append(f"{spc}  {method}:")
            out.append(f"{sub}summary: {yaml_escape(op['summary'])}")
            out.append(f"{sub}operationId: {op['operationId']}")
            out.append(f"{sub}tags:")
            for t in op['tags']:
                out.append(f"{sub}  - {t}")
            if op.get('parameters'):
                out.append(f"{sub}parameters:")
                for prm in op['parameters']:
                    out.append(f"{sub}  - name: {prm['name']}")
                    out.append(f"{sub}    in: {prm['in']}")
                    out.append(f"{sub}    required: {'true' if prm['required'] else 'false'}")
                    out.append(f"{sub}    schema:")
                    out.append(f"{sub}      type: {prm['schema']['type']}")
                    if prm['schema'].get('format'):
                        out.append(f"{sub}      format: {prm['schema']['format']}")
                    out.append(f"{sub}    description: {yaml_escape(prm['description'])}")
            if 'requestBody' in op:
                out.append(f"{sub}requestBody:")
                out.append(f"{sub}  required: true")
                out.append(f"{sub}  content:")
                out.append(f"{sub}    application/json:")
                out.append(f"{sub}      schema:")
                out.append(f"{sub}        $ref: '#/components/schemas/GenericRequest'")
            out.append(f"{sub}responses:")
            for code, resp in op['responses'].items():
                if isinstance(resp, dict) and '$ref' in resp:
                    out.append(f"{sub}  '{code}':")
                    out.append(f"{sub}    $ref: {resp['$ref']}")
                else:
                    out.append(f"{sub}  '{code}':")
                    out.append(f"{sub}    description: {yaml_escape(resp['description'])}")
                    out.append(f"{sub}    content:")
                    out.append(f"{sub}      application/json:")
                    out.append(f"{sub}        schema:")
                    out.append(f"{sub}          $ref: '#/components/schemas/OKEnvelope'")
            if op.get('security'):
                out.append(f"{sub}security:")
                for s in op['security']:
                    out.append(f"{sub}  - bearerAuth: []")
            else:
                out.append(f"{sub}security: []")
            out.append(f"{sub}x-handler: {op['x-handler']}")
            out.append(f"{sub}x-source: {op['x-source']}")
            out.append(f"{sub}x-stub: {op['x-stub']}")
    return out  # list of lines

def build_paths_block(routes):
    """Given list of (svc, method, path, handler, file, ln, public), return path->method->op dict."""
    paths = {}
    for svc, method, pub, handler, file, ln, public in routes:
        op = build_op(method, pub, svc, handler, file, ln, public=public)
        paths.setdefault(pub, {})[method.lower()] = op
    return paths

def render_full(routes):
    auth_routes = [r for r in routes if r[0] == 'api-gateway']
    biz_routes  = [r for r in routes if r[0] != 'api-gateway']

    auth_paths = build_paths_block([(*r, False) for r in auth_routes])
    biz_paths  = build_paths_block([(*r, True)  for r in biz_routes])

    head = ["openapi: 3.0.3",
            "info:",
            "  title: BidWriter Backend API",
            "  version: 1.0.0",
            "  description: |",
            "    HTTP API 真相源。By `scripts/gen-api.py` 自动生成，依据各服务的 `internal/api/*.go`",
            "    与 api-gateway 主入口的路由注册。",
            "",
            "    ⚠️ 此规范目前仅包含 method / path / handler 三元组；请求 / 响应 schema 是占位符，",
            "    在每个 service 接上 swag 注释之前请不要直接用于 SDK 生成。",
            "",
            "    完整覆盖范围：",
            "    - 10 个业务微服务",
            "    - api-gateway 自管的 3 个 auth 端点",
            "    - 共 73 个 public 路由",
            "",
            "servers:",
            "  - url: http://localhost:8080/api/v1",
            "    description: 本地开发（api-gateway）",
            "  - url: https://api.bidwriter.app/api/v1",
            "    description: 生产（占位）",
            "",
            "security:",
            "  - bearerAuth: []",
            "",
            "tags:",
            "  - name: auth",
            "    description: 认证（api-gateway 本地处理）"]
    for svc, tag in TAG.items():
        head.append(f"  - name: {tag}")
        head.append(f"    description: {SVC_DESC.get(svc,'')}（{svc}）")
    head.append("")
    head.append("paths:")
    head += render_paths_block(auth_paths, indent=2)
    head += render_paths_block(biz_paths, indent=2)

    components = [
        "",
        "components:",
        "  securitySchemes:",
        "    bearerAuth:",
        "      type: http",
        "      scheme: bearer",
        "      bearerFormat: JWT",
        "      description: |",
        "        `Authorization: Bearer <jwt>`。",
        "        JWT 由 `POST /api/v1/auth/login` 签发；access TTL 默认 1h，refresh TTL 默认 30d。",
        "        详见 docs/api/authentication.md。",
        "",
        "  responses:",
        "    BadRequest:",
        "      description: 请求参数非法",
        "      content:",
        "        application/json:",
        "          schema:",
        "            $ref: '#/components/schemas/Error'",
        "    Unauthorized:",
        "      description: 未认证 / Token 无效",
        "      content:",
        "        application/json:",
        "          schema:",
        "            $ref: '#/components/schemas/Error'",
        "    NotFound:",
        "      description: 资源不存在",
        "      content:",
        "        application/json:",
        "          schema:",
        "            $ref: '#/components/schemas/Error'",
        "    Internal:",
        "      description: 服务内部错误",
        "      content:",
        "        application/json:",
        "          schema:",
        "            $ref: '#/components/schemas/Error'",
        "",
        "  schemas:",
        "    OKEnvelope:",
        "      type: object",
        "      description: 通用成功响应外壳；具体业务字段见各服务。",
        "      properties:",
        "        code:",
        "          type: string",
        "          example: ok",
        "        data:",
        "          type: object",
        "          additionalProperties: true",
        "        request_id:",
        "          type: string",
        "          format: uuid",
        "",
        "    Error:",
        "      type: object",
        "      description: 统一错误响应。错误码见 docs/api/errors.md。",
        "      properties:",
        "        code:",
        "          type: string",
        "          description: 业务错误码（machine-readable）",
        "          example: invalid_input",
        "        message:",
        "          type: string",
        "          description: 面向开发者的错误描述",
        "        request_id:",
        "          type: string",
        "          format: uuid",
        "        details:",
        "          type: object",
        "          additionalProperties: true",
        "",
        "    GenericRequest:",
        "      type: object",
        "      description: 占位请求体。实际 schema 待 swag 注释补齐后替换。",
        "      additionalProperties: true",
    ]
    return '\n'.join(head + components) + '\n'

def emit_per_service(routes):
    """Write docs/api/services/<svc>.yaml per business service + auth.yaml."""
    OUT_SVC.mkdir(parents=True, exist_ok=True)
    by_svc = {}
    for r in routes:
        svc = r[0]
        if svc == 'api-gateway': continue
        by_svc.setdefault(svc, []).append((*r, True))
    auth_routes = [(*r, False) for r in routes if r[0] == 'api-gateway']

    # auth.yaml
    auth_paths = build_paths_block(auth_routes)
    lines = ["# Auth endpoints (handled locally in api-gateway). Public: no `Authorization` required.",
             "# Auto-generated by scripts/gen-api.py.",
             "paths:"]
    lines += render_paths_block(auth_paths, indent=2)
    (OUT_SVC / 'auth.yaml').write_text('\n'.join(lines) + '\n', encoding='utf-8')

    for svc in sorted(by_svc):
        items = by_svc[svc]
        paths = build_paths_block(items)
        lines = [
            f"# Per-service slice for `{svc}` ({SVC_DESC.get(svc,'')}).",
            "# Auto-generated by scripts/gen-api.py.",
            "paths:",
        ]
        lines += render_paths_block(paths, indent=2)
        (OUT_SVC / f'{svc}.yaml').write_text('\n'.join(lines) + '\n', encoding='utf-8')

def main():
    routes = collect_routes()
    emit_per_service(routes)
    OUT_FULL.parent.mkdir(parents=True, exist_ok=True)
    OUT_FULL.write_text(render_full(routes), encoding='utf-8')
    print(f"wrote {OUT_FULL} ({len(routes)} routes)")

if __name__ == '__main__':
    main()
