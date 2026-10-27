import type { ReactNode } from "react";

/**
 * 利用規約の表示用の小さな Markdown 表示（見出し・段落・箇条書きだけ）。
 * HTML は解釈せず、すべて文字列として表示する（外部のライブラリを読み込まない）。
 */
export function Markdown({ text }: { text: string }) {
  const blocks: ReactNode[] = [];
  let list: string[] = [];
  let para: string[] = [];
  const flush = () => {
    if (para.length > 0) blocks.push(<p key={blocks.length}>{para.join("")}</p>);
    if (list.length > 0)
      blocks.push(
        <ul key={blocks.length}>
          {list.map((item, i) => (
            <li key={i}>{item}</li>
          ))}
        </ul>,
      );
    para = [];
    list = [];
  };
  for (const raw of text.split("\n")) {
    const line = raw.trimEnd();
    const heading = /^(#{1,3})\s+(.*)$/.exec(line);
    const item = /^\s*[-*]\s+(.*)$/.exec(line);
    if (heading) {
      flush();
      const level = heading[1].length;
      const Tag = level === 1 ? "h1" : level === 2 ? "h2" : "h3";
      blocks.push(<Tag key={blocks.length}>{heading[2]}</Tag>);
    } else if (item) {
      if (para.length > 0) flush();
      list.push(item[1]);
    } else if (line === "") {
      flush();
    } else {
      if (list.length > 0) flush();
      para.push(line);
    }
  }
  flush();
  return <div className="markdown">{blocks}</div>;
}
