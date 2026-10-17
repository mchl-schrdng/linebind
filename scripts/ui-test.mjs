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

  await test('the line selects the same clause, restores filtered checks and preserves native anchors', () => {
    const dom = parse(html, { scripts: true });
    const document = dom.window.document;
    const marks = [...document.querySelectorAll('[data-line-check]')];
    assert.equal(marks.length, report.checks.length);
    assert.equal(marks[1].getAttribute('aria-current'), 'true');
    document.querySelector('[data-filter="attention"]').click();
    marks[0].click();
    selected(dom, 0);
    assert.equal(marks[0].getAttribute('aria-current'), 'true');
    assert.equal(document.querySelector('[data-filter="all"]').getAttribute('aria-pressed'),'true');
    assert.equal(dom.window.location.hash,'#check-0');
    const plain = parse(html).window.document;
    assert.equal(plain.querySelector('[data-line-check]').getAttribute('href'),'#check-0');
    assert.equal(plain.querySelector('.definition pre').textContent,report.checks[0].specification);
    assert.equal(plain.querySelector('details:not([open])'),null,'all evidence stays expanded without scripts');
  });

  await test('printing exposes all definitions and restores the reader’s expanded sections', () => {
    const dom=parse(html,{scripts:true}),document=dom.window.document;
    const chosen=document.querySelector('.definition');chosen.open=true;
    const previouslyClosed=[...document.querySelectorAll('details:not([open])')];
    dom.window.dispatchEvent(new dom.window.Event('beforeprint'));
    assert.equal(document.querySelector('details:not([open])'),null);
    dom.window.dispatchEvent(new dom.window.Event('afterprint'));
    assert.equal(chosen.open,true);
    previouslyClosed.forEach(detail=>assert.equal(detail.open,false));
  });

  await test('copy check preserves context and only the selected check evidence, with accessible fallback', async () => {
    const copied = [];
    const dom = parse(html, {scripts:true,clipboard:{writeText:async text=>copied.push(text)}});
    const document=dom.window.document;
    document.querySelector('[data-copy-check="1"]').click();
    await tick();
    const text=copied[0];
    for(const value of ['Single-check excerpt','Contract hash: '+report.contract_hash,'Input digest: '+report.input_digest,'[Crossed] no-debug-marker',report.checks[1].specification])assert.ok(text.includes(value),value);
    assert.ok(!text.includes('[Passed] readme-present'));
    assert.ok(!text.includes('[Not verified] document-readable'));
    const denied=parse(html,{scripts:true});
    const button=denied.window.document.querySelector('[data-copy-check="1"]');
    button.click();await tick();
    const field=denied.window.document.querySelector('textarea');
    assert.equal(field.getAttribute('aria-label'),'Check diagnostic');
    assert.equal(field.value,text);
    field.dispatchEvent(new denied.window.KeyboardEvent('keydown',{key:'Escape',bubbles:true}));
    assert.equal(denied.window.document.activeElement,button);
  });

  await test('real CLI evidence is readable without JavaScript and hostile markup stays text', () => {
    const dom = parse(html);
    const document = dom.window.document;
    assert.equal(document.querySelectorAll('script').length, 1);
    assert.equal(document.querySelectorAll('img, iframe, object, embed, [onerror]').length, 0);
    assert.equal(dom.window.__linebindInjected, undefined);
    assert.equal(document.body.classList.contains('enhanced'), false);
    assert.equal(dom.window.getComputedStyle(document.getElementById('copy-report')).display, 'none');
    assert.equal(dom.window.getComputedStyle(document.querySelector('.check-filters')).display, 'none');
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

  await test('the first required issue opens by default; native links and explicit hashes remain authoritative', async () => {
    const dom = parse(html, { scripts: true });
    const links = dom.window.document.querySelectorAll('.rule-link');
    selected(dom, 1);
    links[0].focus();
    assert.equal(dom.window.document.activeElement, links[0]);
    assert.equal(links[0].tagName, 'A');
    assert.equal(links[0].tabIndex, 0);
    links[0].click();
    selected(dom, 0);
    assert.equal(dom.window.location.hash, '#check-0');
    dom.window.location.hash = '#check-2';
    await tick();
    selected(dom, 2);
    dom.window.location.hash = '#unknown';
    await tick();
    selected(dom, 2);
    selected(parse(html, { scripts: true, hash: '#check-2' }), 2);
    selected(parse(html, { scripts: true, hash: '#unknown' }), 1);
    selected(parse(html, { scripts: true, hash: '#check-0' }), 0);
  });

  await test('attention filtering keeps incomplete advice visible and deep links restore filtered-out checks', async () => {
    const dom = parse(html, { scripts: true, hash: '#check-0' });
    const document = dom.window.document;
    const all = document.querySelector('[data-filter="all"]');
    const attention = document.querySelector('[data-filter="attention"]');
    const links = [...document.querySelectorAll('.rule-link')];
    attention.focus();
    attention.click();
    assert.deepEqual(links.map(link => link.parentElement.hidden), [true, false, false]);
    assert.equal(attention.getAttribute('aria-pressed'), 'true');
    assert.equal(all.getAttribute('aria-pressed'), 'false');
    assert.equal(document.activeElement, attention);
    assert.equal(document.getElementById('filter-status').textContent, 'Showing 2 of 3 checks.');
    selected(dom, 1);
    assert.equal(dom.window.location.hash, '#check-1');
    links[2].click();
    selected(dom, 2);
    all.click();
    selected(dom, 2);
    assert.ok(links.every(link => !link.parentElement.hidden));
    attention.click();
    dom.window.location.hash = '#check-0';
    await tick();
    selected(dom, 0);
    assert.equal(all.getAttribute('aria-pressed'), 'true');
    assert.ok(links.every(link => !link.parentElement.hidden));
  });

  await test('clipboard receives the exact visible diagnostic as plain text', async () => {
    const copied = [];
    const dom = parse(html, { scripts: true, clipboard: { writeText: async text => copied.push(text) } });
    const document = dom.window.document;
    document.querySelector('[data-filter="attention"]').click();
    document.getElementById('copy-report').click();
    await tick();
    assert.deepEqual(copied, [document.getElementById('diagnostic-text').textContent]);
    assert.equal(document.getElementById('copy-status').textContent, 'Diagnostic copied.');
    assert.equal(document.querySelector('textarea'), null);
    assert.ok(copied[0].includes('[Passed] readme-present'), 'filtering must not remove passing checks from the copied evidence');
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

  await test('required issues take priority over earlier advice, while advisory-only issues remain discoverable', () => {
    const proposed = { ...contract, rules: [contract.rules[0], contract.rules[2], contract.rules[1]] };
    writeFileSync(join(repo, '.linebind.yml'), JSON.stringify(proposed, null, 2));
    const priorityHTML = join(temp, 'priority.html');
    const preview = () => spawnSync(resolve(binary), ['preview', '--repo', repo, '--base', 'HEAD', '--html', priorityHTML], { encoding: 'utf8' });
    const required = preview();
    assert.equal(required.status, 1, required.stderr || required.error?.message);
    selected(parse(readFileSync(priorityHTML, 'utf8'), { scripts: true }), 2);
    writeFileSync(join(repo, 'src/input.txt'), 'Allowed source.\n');
    const advisory = preview();
    assert.equal(advisory.status, 0, advisory.stderr || advisory.error?.message);
    const dom = parse(readFileSync(priorityHTML, 'utf8'), { scripts: true });
    selected(dom, 1);
    assert.match(dom.window.document.getElementById('check-1').textContent, /advisory check does not block/);
    writeFileSync(join(repo, '.linebind.yml'), `${JSON.stringify(contract, null, 2)}\n# Candidate policy comment\n`);
  });

  await test('a passing preview stays visibly unapproved, including its copied diagnostic', async () => {
    writeFileSync(join(repo, 'src/input.txt'), 'Allowed source.\n');
    writeFileSync(join(repo, 'api/spec.json'), '{"ok":true}\n');
    const previewJSON = join(temp, 'preview.json');
    const previewHTML = join(temp, 'preview.html');
    const result = spawnSync(resolve(binary), ['preview', '--repo', repo, '--base', 'HEAD', '--format', 'json', '--output', previewJSON, '--html', previewHTML], { encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr || result.error?.message);
    const proposed = JSON.parse(readFileSync(previewJSON, 'utf8'));
    assert.equal(proposed.purpose, 'preview');
    assert.equal(proposed.contract_changes.length, 1);
    assert.equal(proposed.rule_changes, undefined, 'comment-only edits do not change rule definitions');
    const copied = [];
    const dom = parse(readFileSync(previewHTML, 'utf8'), { scripts: true, clipboard: { writeText: async text => copied.push(text) } });
    const document = dom.window.document;
    assert.match(document.title, /preview.*not approved/i);
    assert.match(document.getElementById('preview-title').textContent, /not approved/i);
    assert.notEqual(dom.window.getComputedStyle(document.getElementById('preview-title')).display, 'none');
    assert.equal(document.querySelector('.check-filters'), null, 'all-passing results do not need an attention filter');
    selected(dom, 0);
    document.getElementById('copy-report').click();
    await tick();
    assert.equal(copied.length, 1);
    assert.match(copied[0], /PREVIEW.*NOT APPROVED/);
    assert.match(copied[0], /does not approve the policy change/);
    document.querySelector('[data-copy-check="0"]').click();
    await tick();
    assert.match(copied[1], /PREVIEW.*NOT APPROVED/);
    assert.match(copied[1], /Policy source: candidate/);
    const gate = spawnSync(resolve(binary), ['check', '--repo', repo, '--base', 'HEAD'], { encoding: 'utf8' });
    assert.equal(gate.status, 1, 'preview must not approve the policy change');
  });

  await test('rule changes retain before and after definitions without interpreting them as approval', () => {
    const proposed=structuredClone(contract);
    proposed.rules[1].required=false;
    writeFileSync(join(repo,'.linebind.yml'),JSON.stringify(proposed));
    const target=join(temp,'changed.html');
    const result=spawnSync(resolve(binary),['preview','--repo',repo,'--base','HEAD','--html',target],{encoding:'utf8'});
    assert.equal(result.status,0,result.stderr);
    const doc=parse(readFileSync(target,'utf8')).window.document;
    const diff=doc.querySelector('.rule-diff');
    assert.match(diff.querySelector('summary').textContent,/modified.*no-debug-marker/);
    const sides=diff.querySelectorAll('pre');
    assert.match(sides[0].textContent,/required: true/);
    assert.match(sides[1].textContent,/required: false/);
    assert.match(doc.querySelector('#preview-title').textContent,/not approved/i);
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
