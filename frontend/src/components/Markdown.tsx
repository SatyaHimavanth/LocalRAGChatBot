interface MarkdownProps {
  text: string;
  className?: string;
  /** When true, text may contain pre-formatted HTML (e.g. source badges) that should pass through */
  hasPreformattedHtml?: boolean;
}

export function Markdown({text, className, hasPreformattedHtml}: MarkdownProps) {
  const html = hasPreformattedHtml ? renderWithHtml(text) : renderMarkdown(text);
  return (
    <div
      className={className}
      dangerouslySetInnerHTML={{ __html: html }}
      style={{lineHeight: 1.6}}
    />
  );
}

export function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

// Renders markdown while preserving existing HTML tags (for source badges)
function renderWithHtml(text: string): string {
  // First, extract and protect HTML tags
  const tags: string[] = [];
  const protectedText = text.replace(/<[^>]+>/g, (m) => {
    tags.push(m);
    return `\x00TAG${tags.length - 1}\x00`;
  });

  // Escape the remaining text
  let escaped = escapeHtml(protectedText);

  // Restore HTML tags
  escaped = escaped.replace(/\x00TAG(\d+)\x00/g, (_m, idx) => tags[parseInt(idx)]);

  // Now apply markdown formatting on the escaped text (which has HTML restored)
  return applyMarkdownFormatting(escaped);
}

function renderMarkdown(text: string): string {
  return applyMarkdownFormatting(escapeHtml(text));
}

function applyMarkdownFormatting(html: string): string {
  // Code blocks (```...```) — before inline code
  html = html.replace(/```(\w*)\s*([\s\S]*?)```/g, (_m, _lang, code) => {
    return `<pre style="background:rgba(0,0,0,0.1);border-radius:6px;padding:12px;overflow-x:auto;margin:8px 0;font-size:12px;line-height:1.5"><code>${code.trim()}</code></pre>`;
  });

  // Inline code (`...`)
  html = html.replace(/`([^`]+)`/g, '<code style="background:rgba(0,0,0,0.08);padding:1px 5px;border-radius:3px;font-size:0.9em">$1</code>');

  // Bold
  html = html.replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>");
  html = html.replace(/__(.+?)__/g, "<strong>$1</strong>");

  // Italic
  html = html.replace(/\*(.+?)\*/g, "<em>$1</em>");
  html = html.replace(/_(.+?)_/g, "<em>$1</em>");

  // Strikethrough
  html = html.replace(/~~(.+?)~~/g, "<del>$1</del>");

  // Links - only allow http(s)/mailto so a javascript: or data: URL in the
  // text (from a document, or a response influenced by one) can't execute
  // when clicked; anything else renders as plain text instead of a link.
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (m, label, url) => {
    if (!/^(https?:|mailto:)/i.test(url.trim())) return m;
    return `<a href="${url}" target="_blank" rel="noopener noreferrer" style="color:rgba(99,102,241,0.85);text-decoration:underline">${label}</a>`;
  });

  // Unordered lists
  html = html.replace(/^[\s]*[-*]\s+(.+)$/gm, "<li>$1</li>");

  // Ordered lists
  html = html.replace(/^[\s]*\d+\.\s+(.+)$/gm, "<li>$1</li>");

  // Wrap consecutive <li> in <ul>
  html = html.replace(/((?:<li>.*?<\/li>\n?)+)/g, '<ul style="margin:4px 0;padding-left:20px">$1</ul>');

  // Headers
  html = html.replace(/^### (.+)$/gm, "<h4 style='font-size:14px;font-weight:600;margin:12px 0 4px'>$1</h4>");
  html = html.replace(/^## (.+)$/gm, "<h3 style='font-size:15px;font-weight:600;margin:12px 0 4px'>$1</h3>");
  html = html.replace(/^# (.+)$/gm, "<h2 style='font-size:16px;font-weight:600;margin:12px 0 4px'>$1</h2>");

  // Blockquotes
  html = html.replace(/^&gt;\s*(.+)$/gm, '<blockquote style="border-left:3px solid rgba(99,102,241,0.4);margin:6px 0;padding:4px 12px;color:inherit;opacity:0.7">$1</blockquote>');

  // Horizontal rules
  html = html.replace(/^[-*_]{3,}\s*$/gm, '<hr style="border:none;border-top:1px solid;border-top-color:inherit;opacity:0.1;margin:12px 0">');

  // Line breaks and paragraphs
  html = html.replace(/\n\n+/g, "</p><p style='margin:6px 0'>");
  html = html.replace(/\n/g, "<br/>");

  if (!html.startsWith("<")) {
    html = `<p style='margin:4px 0'>${html}</p>`;
  }

  return html;
}