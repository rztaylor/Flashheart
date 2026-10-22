import { readFile } from "node:fs/promises";
import path from "node:path";
import { chromium } from "@playwright/test";

// Renders the README's "How it works" diagram (FH-188) from its SVG source
// into one PNG per GitHub theme, at twice its size for sharp text. The SVG
// picks its colours with prefers-color-scheme, so each render only changes
// the emulated scheme. Run scripts/readme-diagram.sh.
const [source, outputDir] = process.argv.slice(2);
if (!source || !outputDir) {
  console.error("usage: diagram.mjs <source.svg> <output dir>");
  process.exit(2);
}
const name = path.basename(source, ".svg");
const svg = await readFile(source, "utf8");

const browser = await chromium.launch();
try {
  for (const colorScheme of ["light", "dark"]) {
    const page = await browser.newPage({ colorScheme, deviceScaleFactor: 2 });
    await page.setContent(
      `<!doctype html><style>html,body{margin:0;background:transparent}svg{display:block}</style>${svg}`,
    );
    const output = path.join(outputDir, `${name}-${colorScheme}.png`);
    await page
      .locator("svg")
      .screenshot({ path: output, omitBackground: true });
    console.log(output);
    await page.close();
  }
} finally {
  await browser.close();
}
