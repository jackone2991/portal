import type { ReactNode } from "react";

/**
 * Minimal sanitizing markdown renderer for journal entry bodies (SPEC-05
 * P0.4). No markdown/sanitization dependency exists in `package.json`, so
 * this hand-rolls the smallest safe renderer rather than adding one.
 *
 * It never builds or parses an HTML string at all — the source is tokenized
 * into plain-text runs and a handful of inline marks, and every text run is
 * emitted as a React child (`{text}`). React always renders string children
 * as an inert text node, never as markup, so there is no
 * `dangerouslySetInnerHTML` anywhere in this module and nothing to sanitize
 * after the fact: a body containing `<script>alert(1)</script>` comes out the
 * other end as the literal string, exactly per the P0.4 acceptance criterion.
 *
 * Supported syntax (v1, deliberately small — SPEC-05 §5 "non-goals: no rich
 * text editing, no WYSIWYG, no embeds"): paragraphs (blank-line separated),
 * single newlines as `<br>`, `**bold**`, `*italic*` / `_italic_`, and
 * `` `inline code` ``. Anything else (headings, lists, links, raw HTML) is
 * left as literal text — a strict subset, not a full CommonMark parser.
 */
export function renderMarkdown(source: string): ReactNode {
  const paragraphs = source
    .replace(/\r\n/g, "\n")
    .split(/\n{2,}/)
    .filter((p) => p.trim().length > 0);

  if (paragraphs.length === 0) return null;

  return (
    <>
      {paragraphs.map((paragraph, i) => (
        <p key={i} className={i > 0 ? "mt-3" : undefined}>
          {renderParagraph(paragraph, i)}
        </p>
      ))}
    </>
  );
}

function renderParagraph(paragraph: string, paragraphKey: number): ReactNode[] {
  const lines = paragraph.split("\n");
  const out: ReactNode[] = [];
  lines.forEach((line, i) => {
    if (i > 0) out.push(<br key={`p${paragraphKey}-br${i}`} />);
    out.push(...renderInline(line, `${paragraphKey}-${i}`));
  });
  return out;
}

// Bold / italic / code — matched in one pass, left-to-right, non-overlapping.
// Deliberately not nested (e.g. bold-inside-italic) — v1 scope.
const INLINE_TOKEN = /(\*\*[^*]+\*\*|`[^`]+`|\*[^*]+\*|_[^_]+_)/g;

function renderInline(text: string, lineKey: string): ReactNode[] {
  const out: ReactNode[] = [];
  let lastIndex = 0;
  let key = 0;
  for (const match of text.matchAll(INLINE_TOKEN)) {
    const index = match.index ?? 0;
    if (index > lastIndex) out.push(text.slice(lastIndex, index));
    const token = match[0];
    const k = `${lineKey}-${key++}`;
    if (token.startsWith("**")) {
      out.push(<strong key={k}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith("`")) {
      out.push(
        <code
          key={k}
          className="rounded px-1 py-0.5 text-[0.85em]"
          style={{ background: "var(--tpl-surface-2)" }}
        >
          {token.slice(1, -1)}
        </code>,
      );
    } else {
      out.push(<em key={k}>{token.slice(1, -1)}</em>);
    }
    lastIndex = index + token.length;
  }
  if (lastIndex < text.length) out.push(text.slice(lastIndex));
  return out;
}
