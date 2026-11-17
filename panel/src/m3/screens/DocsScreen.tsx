// 屏幕4 MCP Docs：复用现有 McpDocsView（iframe 嵌入 /mcp-docs-app/）。
import { McpDocsView } from "../../mcpdocs/McpDocsView";
import { EmbedCard } from "../ui";

export function DocsScreen() {
  return (
    <EmbedCard title="MCP Docs">
      <McpDocsView />
    </EmbedCard>
  );
}
