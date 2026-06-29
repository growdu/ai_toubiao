#!/usr/bin/env node
/**
 * mermaid-lint.mjs
 *
 * 校验所有 markdown 文件里的 ```mermaid 代码块能否被 mermaid.js 正常渲染。
 * 用于本地开发与 CI；执行前请先 `npm install`（一次性安装 puppeteer-core + mermaid）。
 *
 * 用法：
 *   node tools/mermaid-lint.mjs              # 校验 docs 与 README
 *   node tools/mermaid-lint.mjs README.md     # 校验指定文件（支持 glob）
 *
 * 环境变量：
 *   MERMAID_LINT_CHROME=/path/to/chrome    自定义 Chrome / Chromium 可执行路径
 *   MERMAID_LINT_TIMEOUT=5000              单块渲染超时（毫秒），默认 5000
 *   MERMAID_LINT_VERBOSE=1                 打印每个块的详细日志
 *
 * 退出码：
 *   0 - 所有 mermaid 块渲染成功
 *   1 - 至少一个块渲染失败
 *   2 - 环境/参数错误（如 Chrome 未找到、无 mermaid 块等）
 */

import { readFileSync, writeFileSync, existsSync } from 'node:fs';
import { resolve, relative } from 'node:path';
import { glob } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';

const ROOT = resolve(new URL('..', import.meta.url).pathname);
const CHROME = process.env.MERMAID_LINT_CHROME
    || findChrome();
const TIMEOUT = parseInt(process.env.MERMAID_LINT_TIMEOUT || '5000', 10);
const VERBOSE = process.env.MERMAID_LINT_VERBOSE === '1';

// ────────────────────────────────────────────────────────────────────────
// 工具函数
// ────────────────────────────────────────────────────────────────────────

function findChrome() {
    // 优先级：环境变量 > Linux 系统 Chrome/Chromium > macOS > Windows
    const candidates = [
        '/usr/bin/google-chrome',
        '/usr/bin/google-chrome-stable',
        '/usr/bin/chromium',
        '/usr/bin/chromium-browser',
        '/snap/bin/chromium',
        '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
        'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
        'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    ];
    for (const p of candidates) {
        if (existsSync(p)) return p;
    }
    return null;
}

function findMermaidPath() {
    // 在 ROOT 和祖先目录找 node_modules/mermaid/dist/mermaid.min.js
    const candidates = [
        resolve(ROOT, 'node_modules/mermaid/dist/mermaid.min.js'),
        resolve(ROOT, '../node_modules/mermaid/dist/mermaid.min.js'),
    ];
    for (const p of candidates) {
        if (existsSync(p)) return p;
    }
    return null;
}

async function extractMermaidBlocks(files) {
    const blocks = [];
    for (const file of files) {
        const md = readFileSync(file, 'utf-8');
        const re = /```mermaid\n([\s\S]*?)```/g;
        let m;
        let idx = 0;
        while ((m = re.exec(md)) !== null) {
            blocks.push({
                file,
                index: idx++,
                line: md.slice(0, m.index).split('\n').length,
                code: m[1].trim(),
            });
        }
    }
    return blocks;
}

function makeHtml(mermaidJsPath, code) {
    return `<!DOCTYPE html>
<html><head><meta charset="utf-8">
<script src="file://${mermaidJsPath}"></script>
</head><body>
<pre class="mermaid">${escapeHtml(code)}</pre>
<script>
  window.__RESULT = { ok: false, err: null };
  try {
    mermaid.initialize({ startOnLoad: false, securityLevel: 'loose' });
    mermaid.run({ querySelector: '.mermaid' }).then(() => {
      window.__RESULT = { ok: true };
    }).catch(e => {
      window.__RESULT = { ok: false, err: String(e && e.message || e) };
    });
  } catch (e) {
    window.__RESULT = { ok: false, err: String(e && e.message || e) };
  }
</script>
</body></html>`;
}

function escapeHtml(s) {
    return s
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}

// ────────────────────────────────────────────────────────────────────────
// 动态 import puppeteer-core（缺失时给出友好提示）
// ────────────────────────────────────────────────────────────────────────
async function loadPuppeteer() {
    try {
        return await import('puppeteer-core');
    } catch (e) {
        console.error('❌ puppeteer-core 未安装。请先运行: npm install');
        console.error('   详细错误:', e.message);
        process.exit(2);
    }
}

// ────────────────────────────────────────────────────────────────────────
// 主流程
// ────────────────────────────────────────────────────────────────────────
async function main() {
    if (!CHROME) {
        console.error('❌ 找不到 Chrome / Chromium。请设置 MERMAID_LINT_CHROME 环境变量。');
        console.error('   例如: MERMAID_LINT_CHROME=/usr/bin/google-chrome node tools/mermaid-lint.mjs');
        process.exit(2);
    }
    const mermaidPath = findMermaidPath();
    if (!mermaidPath) {
        console.error('❌ 找不到 mermaid.min.js。请先运行: npm install');
        process.exit(2);
    }

    const patterns = process.argv.slice(2);
    const fileGlobs = patterns.length > 0 ? patterns : ['docs/**/*.md', 'README.md'];
    const files = [];
    for (const pattern of fileGlobs) {
        for await (const entry of glob(pattern, { cwd: ROOT, ignore: ['**/node_modules/**', '**/.git/**'] })) {
            files.push(resolve(ROOT, entry));
        }
    }
    if (files.length === 0) {
        console.error(`❌ 未匹配到任何文件: ${fileGlobs.join(', ')}`);
        process.exit(2);
    }

    const blocks = await extractMermaidBlocks(files);
    if (blocks.length === 0) {
        console.log('ℹ️  未找到 mermaid 代码块，跳过渲染校验。');
        return;
    }

    console.log(`🔍 发现 ${blocks.length} 个 mermaid 块，开始渲染校验...`);
    if (VERBOSE) console.log(`   Chrome: ${CHROME}\n   Mermaid: ${mermaidPath}`);

    const puppeteer = await loadPuppeteer();
    const browser = await puppeteer.default.launch({
        executablePath: CHROME,
        headless: 'new',
        args: [
            '--no-sandbox',
            '--disable-setuid-sandbox',
            '--disable-dev-shm-usage',
            '--allow-file-access-from-files',
        ],
    });

    let pass = 0, fail = 0;
    const failures = [];

    try {
        for (const blk of blocks) {
            const htmlPath = `/tmp/mermaid-lint-${process.pid}-${blk.file.replace(/\W/g, '_')}-${blk.index}.html`;
            writeFileSync(htmlPath, makeHtml(mermaidPath, blk.code));

            const page = await browser.newPage();
            const errors = [];
            page.on('pageerror', e => errors.push(String(e.message || e)));
            try {
                await page.goto(`file://${htmlPath}`);
                const start = Date.now();
                let result = null;
                while (Date.now() - start < TIMEOUT) {
                    result = await page.evaluate(() => window.__RESULT);
                    if (result && (result.ok || result.err)) break;
                    await new Promise(r => setTimeout(r, 100));
                }
                if (!result) {
                    throw new Error(`渲染超时（>${TIMEOUT}ms）`);
                }
                const svgCount = await page.evaluate(() => document.querySelectorAll('svg').length);

                if (result.ok && svgCount > 0) {
                    pass++;
                    if (VERBOSE) console.log(`  ✓ ${relative(ROOT, blk.file)}:${blk.line}`);
                } else {
                    fail++;
                    failures.push({
                        file: relative(ROOT, blk.file),
                        line: blk.line,
                        code: blk.code.split('\n')[0],
                        err: result.err || '未生成 SVG',
                        pageErrors: errors,
                    });
                }
            } catch (e) {
                fail++;
                failures.push({
                    file: relative(ROOT, blk.file),
                    line: blk.line,
                    code: blk.code.split('\n')[0],
                    err: e.message,
                    pageErrors: errors,
                });
            } finally {
                await page.close();
            }
        }
    } finally {
        await browser.close();
    }

    // 输出结果
    console.log(`\n📊 结果: ${pass} 通过, ${fail} 失败（总计 ${blocks.length}）`);
    if (fail > 0) {
        console.error('\n❌ 失败的 mermaid 块:');
        for (const f of failures) {
            console.error(`  ${f.file}:${f.line}  ${f.code.slice(0, 50)}...`);
            console.error(`    错误: ${f.err}`);
            if (f.pageErrors.length > 0) {
                console.error(`    页面错误: ${f.pageErrors.join(' | ')}`);
            }
        }
        process.exit(1);
    }
    console.log('✅ 所有 mermaid 块均能正常渲染。');
}

main().catch(e => {
    console.error('💥 意外错误:', e);
    process.exit(2);
});