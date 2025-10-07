import { cp, mkdir, rm, stat, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const distDir = path.resolve(__dirname, '../packages/effects-lib/dist');
const targetDir = path.resolve(__dirname, '../../../client/js-effects');

async function ensureDistExists() {
  try {
    const stats = await stat(distDir);
    if (!stats.isDirectory()) {
      throw new Error('effects-lib dist path exists but is not a directory');
    }
  } catch (error) {
    if (error && error.code === 'ENOENT') {
      throw new Error(
        'Expected build output missing. Run "npm --prefix tools/js-effects run build" to generate dist files.'
      );
    }
    throw error;
  }
}

async function syncDist() {
  await ensureDistExists();
  await rm(targetDir, { recursive: true, force: true });
  await mkdir(targetDir, { recursive: true });
  await cp(distDir, targetDir, { recursive: true });
  await writeFile(path.join(targetDir, '.gitkeep'), '');
}

await syncDist();
