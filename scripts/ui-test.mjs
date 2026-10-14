// Optional DOM tests of the actual CLI report. No browser or resource loading.
// npm install --prefix /tmp/linebind-ui-test --no-save --ignore-scripts jsdom@26.1.0
// NODE_PATH=/tmp/linebind-ui-test/node_modules node scripts/ui-test.mjs /path/to/linebind
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const require = createRequire(import.meta.url);
const { JSDOM, ResourceLoader, VirtualConsole } = require('jsdom');
const binary = process.argv[2];
if (!binary) throw new Error('Usage: node scripts/ui-test.mjs /path/to/linebind');
const temp = mkdtempSync(join(tmpdir(), 'linebind-ui-'));
const repo = join(temp, 'repository');
const hostile = '</script><script>window.__linebindInjected=true</script><img src=x onerror="window.__linebindInjected=true">';
const documents = [];
let count = 0;

function test(name, fn) {
  return Promise.resolve().then(fn).then(() => {
    count++;
    process.stdout.write(`PASS ${name}\n`);
  });
}

function parse(html, { scripts = false, hash = '', clipboard, execCommand } = {}) {
  const errors = [];
  const requests = [];
  class NoResources extends ResourceLoader {
    fetch(url) {
      requests.push(url);
      throw new Error('Resource loading is disabled in this test.');
    }
  }
  const virtualConsole = new VirtualConsole();
  virtualConsole.on('jsdomError', error => errors.push(error));
  const dom = new JSDOM(html, {
    url: `https://linebind.invalid/${hash}`,
    runScripts: scripts ? 'dangerously' : undefined,
    resources: new NoResources(),
    virtualConsole,
    beforeParse(window) {
      Object.defineProperty(window, 'isSecureContext', { value: true });
      if (clipboard) Object.defineProperty(window.navigator, 'clipboard', { value: clipboard });
      if (execCommand) window.document.execCommand = execCommand;
      const blocked = () => { throw new Error('Network access is disabled in this test.'); };
      window.fetch = blocked;
      window.XMLHttpRequest = blocked;
      window.WebSocket = blocked;
    },
  });
  documents.push({ dom, errors, requests });
  return dom;
}

function selected(dom, index) {
  const document = dom.window.document;
  const links = [...document.querySelectorAll('.rule-link')];
  const panels = [...document.querySelectorAll('.clause-panel')];
  assert.equal(links.filter(link => link.getAttribute('aria-current') === 'true').length, 1);
  assert.equal(links[index].getAttribute('aria-current'), 'true');
  assert.deepEqual(panels.map(panel => panel.hidden), panels.map((_, i) => i !== index));
}

const tick = () => new Promise(resolve => setTimeout(resolve, 0));

try {
  mkdirSync(join(repo, 'src'), { recursive: true });
  mkdirSync(join(repo, 'api'));
  const contract = {
    version: 1,
    name: 'Report interaction fixture',
    rules: [
      { id: 'readme-present', kind: 'require-files', reason: 'Keep the project introduction.', files: { require: ['README.md'] } },
      { id: 'no-debug-marker', kind: 'forbid-text', reason: hostile, scope: ['src/input.txt'], remediation: hostile, text: { pattern: 'FORBIDDEN' } },
      { id: 'document-readable', kind: 'document', reason: 'Keep the declared value.', required: false, scope: ['api/spec.json'], document: { selector: '/ok', equals: true } },
    ],
  };
  writeFileSync(join(repo, '.linebind.yml'), JSON.stringify(contract, null, 2));
  writeFileSync(join(repo, 'README.md'), 'DOM unit-test fixture.\n');
  writeFileSync(join(repo, 'src/input.txt'), 'Allowed source.\n');
  writeFileSync(join(repo, 'api/spec.json'), '{"ok":true}\n');
  const git = (...args) => execFileSync('git', args, { cwd: repo, stdio: 'pipe' });
  git('init', '--quiet');
  git('config', 'user.name', 'Linebind test');
  git('config', 'user.email', 'test@linebind.invalid');
  git('add', '.');
  git('-c', 'commit.gpgsign=false', 'commit', '--quiet', '-m', 'Reviewed fixture contract');
  writeFileSync(join(repo, 'src/input.txt'), `FORBIDDEN ${hostile}\n`);
  writeFileSync(join(repo, 'api/spec.json'), '{');
  writeFileSync(join(repo, '.linebind.yml'), `${JSON.stringify(contract, null, 2)}\n# Candidate policy comment\n`);
  const jsonPath = join(temp, 'report.json');
  const htmlPath = join(temp, 'report.html');
  const run = spawnSync(resolve(binary), ['check', '--repo', repo, '--base', 'HEAD', '--format', 'json', '--output', jsonPath, '--html', htmlPath], { encoding: 'utf8' });
  assert.equal(run.status, 1, run.stderr || run.error?.message);
  const report = JSON.parse(readFileSync(jsonPath, 'utf8'));
  const html = readFileSync(htmlPath, 'utf8');
  assert.deepEqual(report.checks.map(check => check.status), ['passed', 'crossed', 'unverified']);
  assert.equal(report.contract_changes.length, 1);

  await test('real CLI evidence is readable without JavaScript and hostile markup stays text', () => {
    const dom = parse(html);
    const document = dom.window.document;
    assert.equal(document.querySelectorAll('script').length, 1);
    assert.equal(document.querySelectorAll('img, iframe, object, embed, [onerror]').length, 0);
    assert.equal(dom.window.__linebindInjected, undefined);
    assert.equal(document.body.classList.contains('enhanced'), false);
    assert.equal(dom.window.getComputedStyle(document.getElementById('copy-report')).display, 'none');
    const panels = [...document.querySelectorAll('.clause-panel')];
    assert.equal(panels.length, report.checks.length);
    panels.forEach((panel, i) => {
      const check = report.checks[i];
      assert.equal(panel.hidden, false);
      assert.notEqual(dom.window.getComputedStyle(panel).display, 'none');
      assert.equal(panel.querySelector('h2').textContent, check.id);
      assert.equal(panel.querySelector('.reason').textContent, check.reason);
      for (const finding of check.findings || []) assert.ok(panel.textContent.includes(finding.message));
      for (const diagnostic of check.diagnostics || []) assert.ok(panel.textContent.includes(diagnostic));
    });
    assert.ok(document.querySelector('.contract-changes').textContent.includes('.linebind.yml'));
    const ids = [...document.querySelectorAll('[id]')].map(element => element.id);
    assert.equal(new Set(ids).size, ids.length);
    for (const link of document.querySelectorAll('a[href^="#"]')) assert.ok(document.querySelector(link.getAttribute('href')));
  });

  await test('native links select checks; valid hashes select and unknown hashes preserve selection', async () => {
    const dom = parse(html, { scripts: true });
    const links = dom.window.document.querySelectorAll('.rule-link');
    selected(dom, 0);
    links[1].focus();
    assert.equal(dom.window.document.activeElement, links[1]);
    assert.equal(links[1].tagName, 'A');
    assert.equal(links[1].tabIndex, 0);
    links[1].click();
    selected(dom, 1);
    assert.equal(dom.window.location.hash, '#check-1');
    dom.window.location.hash = '#check-2';
    await tick();
    selected(dom, 2);
    dom.window.location.hash = '#unknown';
    await tick();
    selected(dom, 2);
    selected(parse(html, { scripts: true, hash: '#check-2' }), 2);
    selected(parse(html, { scripts: true, hash: '#unknown' }), 0);
  });

  await test('clipboard receives the exact visible diagnostic as plain text', async () => {
    const copied = [];
    const dom = parse(html, { scripts: true, clipboard: { writeText: async text => copied.push(text) } });
    const document = dom.window.document;
    document.getElementById('copy-report').click();
    await tick();
    assert.deepEqual(copied, [document.getElementById('diagnostic-text').textContent]);
    assert.equal(document.getElementById('copy-status').textContent, 'Diagnostic copied.');
    assert.equal(document.querySelector('textarea'), null);
  });

  await test('denied clipboard uses selected plain-text fallback and restores focus after copying', async () => {
    let copied;
    let dom;
    dom = parse(html, {
      scripts: true,
      clipboard: { writeText: async () => { throw new Error('Permission denied'); } },
      execCommand: command => {
        assert.equal(command, 'copy');
        const field = dom.window.document.querySelector('textarea');
        assert.equal(field.readOnly, true);
        assert.equal(field.selectionStart, 0);
        assert.equal(field.selectionEnd, field.value.length);
        copied = field.value;
        return true;
      },
    });
    const document = dom.window.document;
    const button = document.getElementById('copy-report');
    button.click();
    await tick();
    assert.equal(copied, document.getElementById('diagnostic-text').textContent);
    assert.equal(document.querySelector('textarea'), null);
    assert.equal(document.activeElement, button);
  });

  await test('unavailable copying leaves one accessible selection; Escape closes it and restores focus', async () => {
    const dom = parse(html, { scripts: true });
    const document = dom.window.document;
    const button = document.getElementById('copy-report');
    button.click();
    await tick();
    button.click();
    await tick();
    assert.equal(document.querySelectorAll('textarea').length, 1);
    const field = document.querySelector('textarea');
    assert.equal(field.getAttribute('aria-label'), 'Report diagnostic');
    assert.equal(field.value, document.getElementById('diagnostic-text').textContent);
    assert.equal(document.activeElement, field);
    assert.ok(document.getElementById('copy-status').textContent.includes('press Escape'));
    field.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    assert.equal(document.querySelector('textarea'), null);
    assert.equal(document.activeElement, button);
  });

  for (const { dom, errors, requests } of documents) {
    assert.deepEqual(errors, [], 'Unexpected DOM script errors');
    assert.deepEqual(requests, [], 'Report attempted resource loading');
    assert.equal(dom.window.__linebindInjected, undefined);
  }
  process.stdout.write(`${count} DOM tests passed. No browser or visual QA was performed.\n`);
} finally {
  for (const { dom } of documents) dom.window.close();
  rmSync(temp, { recursive: true, force: true });
}
