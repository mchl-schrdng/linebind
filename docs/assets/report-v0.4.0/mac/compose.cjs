// Render the Mac presentation from unmodified browser captures and CLI stdout.
// Requires Node.js and sharp; it is not part of the product build.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const sharp = require('sharp');
const dir = __dirname;
const xml = s => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
const uri = name => `data:image/${name.endsWith('.png') ? 'png' : 'jpeg'};base64,${fs.readFileSync(path.join(dir, name)).toString('base64')}`;
const pixelScale = 1265 / 1280; // Native browser export: 1280 CSS px -> 1265 image px.
const image = (name, y) => `<image x="0" y="${y}" width="1265" height="712" href="${uri(name)}"/>`;
const report = image('report-top.jpg', 0) + image('report-bottom.jpg', 599 * pixelScale);
const preview = image('preview-top.jpg', 0) + image('preview-bottom.jpg', 720 * pixelScale);
const W = 1600, X = 96, Y = 88, WW = 1408, bar = 54;

function frame(name, title, content, contentHeight, dark = false) {
  const WH = contentHeight + bar, H = Math.ceil(Y * 2 + WH);
  const paper = dark ? '#10121B' : '#F6F5F1';
  const source = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
<title>${xml(title)}</title>
<desc>Mac-style editorial presentation of actual Linebind v0.4.0 fixture evidence.</desc>
<defs>
  <clipPath id="desktop"><rect width="${W}" height="${H}" rx="24"/></clipPath>
  <clipPath id="window"><rect x="${X}" y="${Y}" width="${WW}" height="${WH}" rx="16"/></clipPath>
  <filter id="shadow" x="-20%" y="-20%" width="140%" height="150%"><feDropShadow dx="0" dy="30" stdDeviation="28" flood-color="#040A24" flood-opacity=".5"/></filter>
</defs>
<g clip-path="url(#desktop)">
  <image width="${W}" height="${H}" preserveAspectRatio="xMidYMid slice" href="${uri('wallpaper.png')}"/>
  <rect x="${X}" y="${Y}" width="${WW}" height="${WH}" rx="16" fill="${paper}" filter="url(#shadow)"/>
  <g clip-path="url(#window)">
    <rect x="${X}" y="${Y}" width="${WW}" height="${WH}" fill="${paper}"/>
    <rect x="${X}" y="${Y}" width="${WW}" height="${bar}" fill="${dark ? '#1C1E29' : '#E9E8E6'}"/>
    <path d="M${X} ${Y + bar}H${X + WW}" stroke="${dark ? '#343641' : '#D8D7D4'}"/>
    <circle cx="${X + 27}" cy="${Y + 27}" r="7" fill="#FF5F57" stroke="#E0453E" stroke-width=".6"/>
    <circle cx="${X + 50}" cy="${Y + 27}" r="7" fill="#FEBC2E" stroke="#DDA123" stroke-width=".6"/>
    <circle cx="${X + 73}" cy="${Y + 27}" r="7" fill="#28C840" stroke="#1AAB29" stroke-width=".6"/>
    <text x="800" y="${Y + 34}" text-anchor="middle" font-family="Helvetica,Arial,sans-serif" font-size="17" fill="${dark ? '#B8BAC8' : '#62646A'}">${xml(title)}</text>
    <g transform="translate(${X} ${Y + bar})">${content}</g>
  </g>
  <rect x="${X + .5}" y="${Y + .5}" width="${WW - 1}" height="${WH - 1}" rx="16" fill="none" stroke="#FFFFFF" stroke-opacity=".4"/>
</g></svg>`;
  return sharp(Buffer.from(source)).png().toFile(path.join(dir, name + '.png'));
}

function browserContent(captures, {x = 0, y = 0, width = 1250, height}) {
  const scale = WW / width;
  return {height: height * scale, content: `<svg width="${WW}" height="${height * scale}" viewBox="${x} ${y} ${width} ${height}"><rect x="${x}" y="${y}" width="${width}" height="${height}" fill="#F6F5F1"/>${captures}</svg>`};
}

async function main() {
  for (const name of ['report-top.jpg', 'report-bottom.jpg', 'preview-top.jpg', 'preview-bottom.jpg']) {
    const m = await sharp(path.join(dir, name)).metadata();
    assert.equal(m.width, 1265); assert.equal(m.height, 712);
  }
  // The shared 120px strip must align before composing a continuous full report.
  const overlapA = await sharp(path.join(dir, 'report-top.jpg')).extract({left: 0, top: 592, width: 1248, height: 120}).removeAlpha().raw().toBuffer();
  const overlapB = await sharp(path.join(dir, 'report-bottom.jpg')).extract({left: 0, top: 0, width: 1248, height: 120}).removeAlpha().raw().toBuffer();
  const meanError = overlapA.reduce((sum, v, i) => sum + Math.abs(v - overlapB[i]), 0) / overlapA.length;
  assert(meanError < 5, `Browser captures do not align: ${meanError}`);
  const views = [
    ['line-mac', 'react-storefront · Linebind — contract line', browserContent(image('report-top.jpg', 0), {x: 32 * pixelScale, y: 120 * pixelScale, width: 1200 * pixelScale, height: 450 * pixelScale})],
    ['report-mac', 'react-storefront · Linebind report', browserContent(report, {height: 1319.1640625 * pixelScale})],
    ['preview-mac', 'react-storefront · Linebind preview', browserContent(preview, {height: 1356 * pixelScale})],
  ];
  for (const [name, title, view] of views) await frame(name, title, view.content, view.height);
  const stdout = fs.readFileSync(path.join(dir, 'cli.txt'), 'utf8').trimEnd();
  const command = fs.readFileSync(path.join(dir, 'cli-command.txt'), 'utf8').trim();
  const exit = fs.readFileSync(path.join(dir, 'cli-exit.txt'), 'utf8').trim();
  assert.equal(exit, '1'); assert(stdout.startsWith('Linebind v0.4.0 — Crossed'));
  const lines = [];
  for (let line of stdout.split('\n')) {
    while (line.length > 94) {
      const split = line.lastIndexOf(' ', 94);
      assert(split > 0);
      lines.push(line.slice(0, split)); line = line.slice(split + 1);
    }
    lines.push(line);
  }
  assert.equal(lines.join(' ').replace(/\s+/g, ' '), stdout.replace(/\s+/g, ' '));
  const color = s => s.startsWith('Linebind') || s.startsWith('[Crossed]') ? '#FF9B83' : s.startsWith('Comparison:') || s.startsWith('worktree:') || s.startsWith('Repository:') ? '#A5ABBC' : '#E8EAF0';
  let body = `<text x="44" y="60" fill="#B5C9FF" font-family="Menlo,monospace" font-size="22">$ ${xml(command)}</text>`;
  lines.forEach((line, i) => { body += `<text x="44" y="${119 + i * 34}" fill="${color(line)}" font-family="Menlo,monospace" font-size="22" xml:space="preserve">${xml(line)}</text>`; });
  const end = 148 + lines.length * 34;
  body += `<path d="M44 ${end}H1364" stroke="#313440"/><text x="44" y="${end + 39}" fill="#959BAE" font-family="Menlo,monospace" font-size="17">linebind v0.4.0 · captured stdout</text><text x="1364" y="${end + 39}" text-anchor="end" fill="#FF9B83" font-family="Menlo,monospace" font-size="17">exit ${exit}</text>`;
  await frame('terminal-mac', 'react-storefront — zsh', body, end + 68, true);
  console.log(`Rendered four Mac compositions. Capture overlap mean error: ${meanError.toFixed(3)} / 255.`);
}
main().catch(error => { console.error(error); process.exitCode = 1; });
