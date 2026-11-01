import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import path from 'node:path';

// Highest root-module release tag; `redis/v*` tags belong to the nested module.
export function latestRelease(): string {
  try {
    const out = execFileSync('git', ['tag', '--list', 'v[0-9]*', '--sort=-v:refname'], {
      stdio: ['ignore', 'pipe', 'ignore'],
    })
      .toString()
      .trim()
      .split('\n')[0];
    if (out) return out;
  } catch {}
  try {
    const manifest = JSON.parse(
      readFileSync(path.join(process.cwd(), '..', '.release-please-manifest.json'), 'utf8'),
    ) as Record<string, string>;
    return `v${manifest['.']}`;
  } catch {
    return '';
  }
}
