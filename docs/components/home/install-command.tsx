'use client';
import { Check, Copy } from 'lucide-react';
import { useState } from 'react';

const command = 'go get github.com/grx-gql/grx@latest';

export function InstallCommand() {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  }

  return (
    <button
      type="button"
      onClick={copy}
      aria-label="Copy install command"
      className="group inline-flex max-w-full items-center gap-3 rounded-md border border-fd-border bg-fd-card px-4 py-3 font-mono text-sm text-fd-foreground transition hover:border-grx-400/60"
    >
      <span className="select-none text-accent">$</span>
      <span className="truncate">{command}</span>
      <span className="ms-1 text-fd-muted-foreground transition group-hover:text-fd-foreground">
        {copied ? <Check className="size-4 text-emerald-500" /> : <Copy className="size-4" />}
      </span>
    </button>
  );
}
