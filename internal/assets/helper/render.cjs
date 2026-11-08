// Reads one JSON request on stdin and writes a PNG to stdout.
//   {"mode":"math","tex":"...","display":true,"color":"#cdd6f4","exPx":8}
//   {"mode":"svg","svg":"<svg...>","scale":2}
const { Resvg } = require('@resvg/resvg-js');

function readStdin() {
  return new Promise((resolve) => {
    const chunks = [];
    process.stdin.on('data', (c) => chunks.push(c));
    process.stdin.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
  });
}

function mathToSVG(tex, display, color, exPx) {
  const { mathjax } = require('mathjax-full/js/mathjax.js');
  const { TeX } = require('mathjax-full/js/input/tex.js');
  const { SVG } = require('mathjax-full/js/output/svg.js');
  const { liteAdaptor } = require('mathjax-full/js/adaptors/liteAdaptor.js');
  const { RegisterHTMLHandler } = require('mathjax-full/js/handlers/html.js');
  const { AllPackages } = require('mathjax-full/js/input/tex/AllPackages.js');

  const adaptor = liteAdaptor();
  RegisterHTMLHandler(adaptor);
  const doc = mathjax.document('', {
    InputJax: new TeX({ packages: AllPackages }),
    OutputJax: new SVG({ fontCache: 'none' }),
  });
  const node = doc.convert(tex, { display, em: exPx * 2, ex: exPx, containerWidth: 4000 });
  let svg = adaptor.innerHTML(node);
  if (svg.includes('data-mjx-error')) {
    const m = svg.match(/data-mjx-error="([^"]*)"/);
    throw new Error(m ? m[1] : 'TeX error');
  }
  // MathJax sizes the SVG in ex; resvg needs pixels.
  svg = svg.replace(/(width|height)="([\d.]+)ex"/g, (_, attr, v) => `${attr}="${(parseFloat(v) * exPx).toFixed(2)}"`);
  svg = svg.replace(/currentColor/g, color);
  return svg;
}

(async () => {
  try {
    const req = JSON.parse(await readStdin());
    let svg, opts = {};
    if (req.mode === 'math') {
      svg = mathToSVG(req.tex, !!req.display, req.color || '#000', req.exPx || 8);
    } else if (req.mode === 'svg') {
      svg = req.svg;
      opts.fitTo = { mode: 'zoom', value: req.scale || 1 };
    } else {
      throw new Error('unknown mode ' + req.mode);
    }
    const png = new Resvg(svg, { ...opts, font: { loadSystemFonts: true }, background: 'rgba(0,0,0,0)' }).render().asPng();
    process.stdout.write(png);
  } catch (e) {
    process.stderr.write(String(e && e.message || e));
    process.exit(1);
  }
})();
