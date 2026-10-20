// Official-SDK MCP client check for `kron serve-mcp`.
//
// Unlike the hand-rolled stdio probes, this drives kron through the SAME
// TypeScript SDK that VSCode / Cursor / Claude-compatible clients use
// (@modelcontextprotocol/sdk v1.30 is already on this machine as a dependency
// of the filesystem MCP server), so it validates the real client path:
// initialize handshake, tools/list, JSON-schema arguments and tools/call.
//
// It runs against a TEMP COPY of the project so write tools cannot dirty the
// release-shaped environment or the committed fixture.
//
// Usage (from the repo root):
//   node frontend/wails/scripts/mcp-sdk-client.mjs
//   node frontend/wails/scripts/mcp-sdk-client.mjs --kron <app>/kron.exe --project <dir>
//
// SDK resolution: MCP_SDK_DIR env var, else the SDK bundled with the local
// filesystem MCP server, else a normal node_modules lookup relative to cwd.
import {createRequire} from 'node:module';
import {pathToFileURL} from 'node:url';
import {existsSync, mkdtempSync, cpSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join, resolve, dirname} from 'node:path';
import {fileURLToPath} from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, '..', '..', '..');

function argValue(name, fallback) {
    const index = process.argv.indexOf(`--${name}`);
    if (index !== -1 && process.argv[index + 1] !== undefined) {
        return process.argv[index + 1];
    }
    return fallback;
}

const kronExe = resolve(argValue('kron', join(repoRoot, 'kron-mcp-test', 'app', 'kron.exe')));
const sourceProject = resolve(argValue('project', join(repoRoot, 'kron-mcp-test', 'project')));

function loadSdk() {
    const candidates = [
        process.env.MCP_SDK_DIR,
        'D:/AITools/mcps/filesystem/node_modules/@modelcontextprotocol/sdk',
        join(repoRoot, 'node_modules', '@modelcontextprotocol/sdk'),
    ].filter((candidate) => typeof candidate === 'string' && candidate.length > 0);
    for (const candidate of candidates) {
        const base = join(candidate, 'package.json');
        if (!existsSync(base)) continue;
        const sdkRequire = createRequire(pathToFileURL(base));
        try {
            return {
                sdkDir: candidate,
                Client: sdkRequire('@modelcontextprotocol/sdk/client/index.js').Client,
                StdioClientTransport:
                    sdkRequire('@modelcontextprotocol/sdk/client/stdio.js').StdioClientTransport,
            };
        } catch (error) {
            console.error(`mcp-sdk-client: SDK at ${candidate} failed to load: ${error.message}`);
        }
    }
    throw new Error(
        'mcp-sdk-client: @modelcontextprotocol/sdk not found; set MCP_SDK_DIR to its package directory',
    );
}

function assert(condition, message) {
    if (!condition) throw new Error(message);
}

async function main() {
    if (!existsSync(kronExe)) throw new Error(`missing kron executable: ${kronExe}`);
    if (!existsSync(sourceProject)) throw new Error(`missing project: ${sourceProject}`);

    const {Client, StdioClientTransport} = loadSdk();
    const projectCopy = mkdtempSync(join(tmpdir(), 'kron-sdk-client-'));
    cpSync(sourceProject, projectCopy, {recursive: true});
    console.log(`kron: ${kronExe}`);
    console.log(`project (temp copy): ${projectCopy}`);

    const client = new Client({name: 'kron-sdk-client', version: '0.0.0'}, {capabilities: {}});
    const transport = new StdioClientTransport({
        command: kronExe,
        args: ['serve-mcp', '-CWD', projectCopy],
        stderr: 'pipe',
    });

    const failures = [];
    const pass = (message) => console.log(`PASS: ${message}`);
    const fail = (message) => {
        failures.push(message);
        console.log(`FAIL: ${message}`);
    };

    try {
        await client.connect(transport);
        pass('initialize handshake via official SDK');

        const {tools} = await client.listTools();
        const names = tools.map((tool) => tool.name).sort();
        const expected = [
            'kron_add', 'kron_assume_check', 'kron_assume_create', 'kron_assume_delete',
            'kron_assume_get', 'kron_assume_list', 'kron_assume_update', 'kron_delete',
            'kron_get', 'kron_impact', 'kron_init', 'kron_intent_density', 'kron_lint',
            'kron_list', 'kron_restore', 'kron_stale', 'kron_tree', 'kron_update',
        ];
        if (JSON.stringify(names) === JSON.stringify(expected)) {
            pass(`tools/list returned all ${names.length} tools with schemas`);
        } else {
            fail(`tools/list mismatch: ${JSON.stringify(names)}`);
        }

        const call = async (name, args) => {
            const result = await client.callTool({name, arguments: args});
            const payload = result.structuredContent;
            assert(payload !== undefined, `${name} returned no structuredContent`);
            return payload;
        };

        const list = await call('kron_list', {});
        (list.intents.length === 8 ? pass : fail)(`kron_list -> ${list.intents.length} intents`);

        const tree = await call('kron_tree', {});
        (tree.root.children.length === 2 ? pass : fail)(`kron_tree -> ${tree.root.children.length} top-level groups`);

        const got = await call('kron_get', {slug: 'auth/jwt'});
        (JSON.stringify(got.intent.frontmatter.symbol).includes('auth.SignIn')
            ? pass
            : fail)('kron_get(auth/jwt) exposes frontmatter symbols');

        const lint = await call('kron_lint', {});
        (lint.summary.errors === 1 && lint.summary.warnings === 1
            ? pass
            : fail)(`kron_lint -> ${lint.summary.errors} errors / ${lint.summary.warnings} warnings`);

        await call('kron_stale', {});
        await call('kron_impact', {slug: 'auth/jwt'});
        await call('kron_intent_density', {});
        const assumptions = await call('kron_assume_list', {});
        (assumptions.assumptions.length === 2 ? pass : fail)(`kron_assume_get/list -> ${assumptions.assumptions.length} assumptions`);
        await call('kron_assume_get', {id: 'single-region'});
        await call('kron_assume_check', {file_path: 'src/auth/jwt.go'});

        // Write path (temp copy only).
        await call('kron_add', {slug: 'sdk-smoke', symbol: 'pkg.Smoke'});
        await call('kron_get', {slug: 'sdk-smoke'});
        await call('kron_update', {slug: 'sdk-smoke', status: 'active'});
        await call('kron_delete', {slug: 'sdk-smoke'});
        await call('kron_restore', {slug: 'sdk-smoke'});
        await call('kron_init', {});
        await call('kron_assume_create', {id: 'sdk-assumption', text: 'sdk smoke', default_severity: 'soft'});
        await call('kron_assume_update', {id: 'sdk-assumption', text: 'sdk smoke updated'});
        await call('kron_assume_delete', {id: 'sdk-assumption'});
        pass('write tools (add/get/update/delete/restore/init/assume_*) via SDK');

        // Tool-level error: SDK returns isError instead of throwing.
        const missing = await client.callTool({name: 'kron_get', arguments: {slug: 'does/not/exist'}});
        (missing.isError === true
            ? pass
            : fail)('tool error surfaces as isError=true through the SDK');
    } catch (error) {
        fail(`unexpected SDK/client error: ${error.message}`);
    } finally {
        try { await client.close(); } catch { /* already closed */ }
        rmSync(projectCopy, {recursive: true, force: true});
    }

    if (failures.length > 0) {
        console.log(`FAILED (${failures.length})`);
        process.exitCode = 1;
        return;
    }
    console.log('ALL PASS (official MCP SDK)');
}

main().catch((error) => {
    console.error(`mcp-sdk-client: ${error.message}`);
    process.exitCode = 1;
});
