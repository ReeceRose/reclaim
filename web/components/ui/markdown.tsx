import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Minimal Markdown renderer for release notes.
 *
 * Deliberately not a full CommonMark implementation: the only Markdown this
 * app renders is `CHANGELOG.md`, whose shape `scripts/release.sh` pins to
 * headings, flat bullet lists, paragraphs, fenced code, and inline
 * code/bold/italic/links. Rendering to React elements rather than an HTML
 * string keeps release notes off `dangerouslySetInnerHTML`.
 */

const INLINE =
  /(`[^`\n]+`)|(\*\*[^*\n]+\*\*)|(\[[^\]\n]+\]\([^)\s]+\))|(\*[^*\n]+\*)/g;

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  let last = 0;
  let match: RegExpExecArray | null;
  let i = 0;

  INLINE.lastIndex = 0;
  match = INLINE.exec(text);
  while (match !== null) {
    if (match.index > last) nodes.push(text.slice(last, match.index));
    const token = match[0];
    const key = `${keyPrefix}-${i++}`;

    if (token.startsWith("`")) {
      nodes.push(
        <code
          key={key}
          className="rounded bg-surface-3 px-1 py-px font-mono text-[0.85em]"
        >
          {token.slice(1, -1)}
        </code>,
      );
    } else if (token.startsWith("**")) {
      nodes.push(
        <strong key={key} className="font-semibold text-text">
          {token.slice(2, -2)}
        </strong>,
      );
    } else if (token.startsWith("[")) {
      const split = token.indexOf("](");
      nodes.push(
        <a
          key={key}
          href={token.slice(split + 2, -1)}
          target="_blank"
          rel="noreferrer"
          className="text-brand hover:underline"
        >
          {token.slice(1, split)}
        </a>,
      );
    } else {
      nodes.push(<em key={key}>{token.slice(1, -1)}</em>);
    }

    last = match.index + token.length;
    match = INLINE.exec(text);
  }

  if (last < text.length) nodes.push(text.slice(last));
  return nodes;
}

const HEADING_CLASS: Record<number, string> = {
  1: "text-base font-semibold text-text mt-5 mb-2",
  2: "text-sm font-semibold text-text mt-5 mb-2",
  3: "text-xs uppercase tracking-widest text-muted-dim font-bold mt-5 mb-2",
  4: "text-sm font-semibold text-text mt-4 mb-1.5",
  5: "text-sm font-semibold text-text mt-4 mb-1.5",
  6: "text-sm font-semibold text-text mt-4 mb-1.5",
};

type Block =
  | { kind: "heading"; level: number; text: string }
  | { kind: "para"; text: string }
  | { kind: "list"; items: string[] }
  | { kind: "code"; text: string };

function parse(source: string): Block[] {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const blocks: Block[] = [];
  let para: string[] = [];
  let list: string[] = [];

  const flushPara = () => {
    if (para.length) blocks.push({ kind: "para", text: para.join(" ") });
    para = [];
  };
  const flushList = () => {
    if (list.length) blocks.push({ kind: "list", items: list });
    list = [];
  };
  const flush = () => {
    flushPara();
    flushList();
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    if (line.trimStart().startsWith("```")) {
      flush();
      const body: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trimStart().startsWith("```")) {
        body.push(lines[i]);
        i++;
      }
      blocks.push({ kind: "code", text: body.join("\n") });
      continue;
    }

    if (!line.trim()) {
      flush();
      continue;
    }

    const heading = /^(#{1,6}) +(.*)$/.exec(line);
    if (heading) {
      flush();
      blocks.push({
        kind: "heading",
        level: heading[1].length,
        text: heading[2].trim(),
      });
      continue;
    }

    const bullet = /^\s*[-*+] +(.*)$/.exec(line);
    if (bullet) {
      flushPara();
      list.push(bullet[1]);
      continue;
    }

    // A continuation line under a bullet belongs to that bullet.
    if (list.length && /^\s+\S/.test(line)) {
      list[list.length - 1] += ` ${line.trim()}`;
      continue;
    }

    flushList();
    para.push(line.trim());
  }

  flush();
  return blocks;
}

export function Markdown({
  source,
  className,
}: {
  source: string;
  className?: string;
}) {
  const blocks = parse(source);

  return (
    <div className={cn("text-sm text-muted-fg leading-relaxed", className)}>
      {blocks.map((block, i) => {
        const key = `${block.kind}-${i}`;
        switch (block.kind) {
          case "heading": {
            const Tag = `h${Math.min(block.level + 1, 6)}` as "h2";
            return (
              <Tag
                key={key}
                className={cn(HEADING_CLASS[block.level], "first:mt-0")}
              >
                {renderInline(block.text, key)}
              </Tag>
            );
          }
          case "list":
            return (
              <ul key={key} className="my-2 flex flex-col gap-1.5 pl-1">
                {block.items.map((item, j) => {
                  // Blocks are derived from an immutable string, so position is
                  // a stable identity here.
                  const itemKey = `${key}-${j}`;
                  return (
                    <li key={itemKey} className="flex gap-2.5">
                      <span
                        aria-hidden
                        className="mt-[0.55em] h-1 w-1 shrink-0 rounded-full bg-muted-dim"
                      />
                      <span className="min-w-0">
                        {renderInline(item, itemKey)}
                      </span>
                    </li>
                  );
                })}
              </ul>
            );
          case "code":
            return (
              <pre
                key={key}
                className="my-3 overflow-x-auto rounded-lg border border-line-soft bg-surface-2 p-3 font-mono text-xs text-text"
              >
                <code>{block.text}</code>
              </pre>
            );
          default:
            return (
              <p key={key} className="my-2 first:mt-0">
                {renderInline(block.text, key)}
              </p>
            );
        }
      })}
    </div>
  );
}
