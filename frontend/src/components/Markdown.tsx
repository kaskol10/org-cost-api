import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { normalizeMarkdownTables } from "../utils/markdown";

interface Props {
  children: string;
  className?: string;
}

export default function Markdown({ children, className = "md-content" }: Props) {
  const text = normalizeMarkdownTables(children);
  return (
    <div className={className}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{text}</ReactMarkdown>
    </div>
  );
}
